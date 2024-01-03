package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"

	"github.com/dagger/dagger/cmd/codegen/introspection"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
	dagintro "github.com/vito/dagql/introspection"
	"github.com/vito/dagql/ioctx"
	"github.com/vito/progrock"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	modMetaDirPath    = "/.daggermod"
	modMetaOutputPath = "output.json"

	ModuleName = "daggercore"
)

/*
ModDeps represents a set of dependencies for a module or for a caller depending on a
particular set of modules to be served.
*/
type ModDeps struct {
	Mods []Mod // TODO hide

	// should not be read directly, call Schema and SchemaIntrospectionJSON instead
	lazilyLoadedSchema            *dagql.Server
	lazilyLoadedIntrospectionJSON string
	loadSchemaErr                 error
	loadSchemaLock                sync.Mutex
}

func NewModDeps(mods []Mod) *ModDeps {
	return &ModDeps{
		Mods: mods,
	}
}

func (d *ModDeps) Prepend(mods ...Mod) *ModDeps {
	deps := append(mods, d.Mods...)
	return NewModDeps(deps)
}

func (d *ModDeps) Append(mods ...Mod) *ModDeps {
	deps := append([]Mod{}, d.Mods...)
	deps = append(deps, mods...)
	return NewModDeps(deps)
}

// The combined schema exposed by each mod in this set of dependencies
func (d *ModDeps) Schema(ctx context.Context, root *Query) (*dagql.Server, error) {
	schema, _, err := d.lazilyLoadSchema(ctx, root)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

// The introspection json for combined schema exposed by each mod in this set of dependencies
func (d *ModDeps) SchemaIntrospectionJSON(ctx context.Context, root *Query) (string, error) {
	_, introspectionJSON, err := d.lazilyLoadSchema(ctx, root)
	if err != nil {
		return "", err
	}
	return introspectionJSON, nil
}

func schemaIntrospectionJSON(ctx context.Context, dag *dagql.Server) (json.RawMessage, error) {
	data, err := dag.Query(ctx, introspection.Query, nil)
	if err != nil {
		return nil, fmt.Errorf("introspection query failed: %w", err)
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal introspection result: %w", err)
	}
	return json.RawMessage(jsonBytes), nil
}

func (d *ModDeps) lazilyLoadSchema(ctx context.Context, root *Query) (loadedSchema *dagql.Server, loadedIntrospectionJSON string, rerr error) {
	d.loadSchemaLock.Lock()
	defer d.loadSchemaLock.Unlock()
	if d.lazilyLoadedSchema != nil {
		return d.lazilyLoadedSchema, d.lazilyLoadedIntrospectionJSON, nil
	}
	if d.loadSchemaErr != nil {
		return nil, "", d.loadSchemaErr
	}
	defer func() {
		d.lazilyLoadedSchema = loadedSchema
		d.lazilyLoadedIntrospectionJSON = loadedIntrospectionJSON
		d.loadSchemaErr = rerr
	}()

	dag := dagql.NewServer[*Query](root)
	// dag.Cache = root.Cache // TODO figure out proper cache sharing
	dag.RecordTo(TelemetryFunc())

	dagintro.Install[*Query](dag)

	for _, mod := range d.Mods {
		log.Println("!!!!!! INSTALLING", mod.Name())
		err := mod.Install(ctx, dag)
		if err != nil {
			log.Println("!!!!!! INSTALLING", mod.Name(), "POOP", err)
			return nil, "", fmt.Errorf("failed to get schema for module %q: %w", mod.Name(), err)
		}
	}

	introspectionJSON, err := schemaIntrospectionJSON(ctx, dag)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get schema introspection JSON: %w", err)
	}

	return dag, string(introspectionJSON), nil
}

// Search the deps for the given type def, returning the ModType if found. This does not recurse
// to transitive dependencies; it only returns types directly exposed by the schema of the top-level
// deps.
func (d *ModDeps) ModTypeFor(ctx context.Context, typeDef *TypeDef) (ModType, bool, error) {
	for _, mod := range d.Mods {
		modType, ok, err := mod.ModTypeFor(ctx, typeDef, false)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get type from mod %q: %w", mod.Name(), err)
		}
		if !ok {
			continue
		}
		return modType, true, nil
	}
	return nil, false, nil
}

func isIntrospection(id *idproto.ID) bool {
	if id.Parent == nil {
		return id.Field == "__schema"
	} else {
		return isIntrospection(id.Parent)
	}
}

func TelemetryFunc() dagql.TelemetryFunc {
	return func(ctx context.Context, id *idproto.ID) (context.Context, func(error)) {
		if isIntrospection(id) {
			return ctx, func(error) {}
		}

		rec := progrock.FromContext(ctx)

		id = id.Canonical() // TODO decice

		dig, err := id.Digest()
		if err != nil {
			slog.Warn("failed to digest id", "id", id.Display(), "err", err)
			return ctx, func(error) {}
		}
		payload, err := anypb.New(id)
		if err != nil {
			slog.Warn("failed to anypb.New(id)", "id", id.Display(), "err", err)
			return ctx, func(error) {}
		}
		vtx := rec.Vertex(dig, id.Field, progrock.Internal())
		ctx = ioctx.WithStdout(ctx, vtx.Stdout())
		ctx = ioctx.WithStderr(ctx, vtx.Stderr())

		// send ID payload to the frontend
		vtx.Meta("id", payload)

		// group any future vertices (e.g. from Buildkit) under this one
		rec = rec.WithGroup(id.Field, progrock.WithGroupID(dig.String()))
		ctx = progrock.ToContext(ctx, rec)

		return ctx, func(rerr error) {
			if rerr != nil {
				log.Println("!!! ID ERRORED", truncate(id.Display(), 200), rerr)
			} else {
				log.Println("!!! ID OK", truncate(id.Display(), 200))
			}
			vtx.Done(rerr)
			rec.Complete()
		}
	}
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}

	if length < 5 {
		return s[:length]
	}

	prefixLength := (length - 3) / 2
	suffixLength := length - 3 - prefixLength

	return s[:prefixLength] + "..." + s[len(s)-suffixLength:]
}
