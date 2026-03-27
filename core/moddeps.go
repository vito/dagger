package core

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/dagger/dagger/dagql"
)

const (
	modMetaDirPath    = "/.daggermod"
	modMetaOutputPath = "output.json"
	modMetaErrorPath  = "error"

	ModuleName = "daggercore"
)

var TypesToIgnoreForModuleIntrospection = []string{"Host"}

// SchemaBuilder lazily constructs a dagql server from a set of modules
// with per-module install policy. It is used both for a module's own
// dependency graph and for the set of modules served to a client session.
//
// SchemaBuilder is immutable: all builder methods return a new instance.
// The server and introspection results are lazily computed and cached on
// first access.
type SchemaBuilder struct {
	root    *Query
	entries []modDepEntry

	// hideCoreAPI, when true, causes the client-facing server to omit
	// core API fields from the Query root (container, directory, etc.).
	// Core types remain installed so return types and method chaining
	// still work; only the Query-level entry points are stripped.
	hideCoreAPI bool

	// lazy cache — computed once per instance, never invalidated
	lazilyLoadedServer *dagql.Server
	loadSchemaErr      error
	loadSchemaLock     sync.Mutex
}

type modDepEntry struct {
	mod  Mod
	opts InstallOpts
}

// NewSchemaBuilder creates a SchemaBuilder from a list of modules, each
// installed with default (zero) InstallOpts.
func NewSchemaBuilder(root *Query, mods []Mod) *SchemaBuilder {
	entries := make([]modDepEntry, len(mods))
	for i, m := range mods {
		entries[i] = modDepEntry{mod: m}
	}
	return &SchemaBuilder{
		root:    root,
		entries: entries,
	}
}

// Clone returns a shallow copy with the same entries.
func (b *SchemaBuilder) Clone() *SchemaBuilder {
	return &SchemaBuilder{
		root:        b.root,
		entries:     slices.Clone(b.entries),
		hideCoreAPI: b.hideCoreAPI,
	}
}

// WithHideCoreAPI returns a new SchemaBuilder that omits core API fields
// from the Query root on the client-facing server. Core types (Container,
// Directory, etc.) remain in the schema so that return types and method
// chaining still work.
func (b *SchemaBuilder) WithHideCoreAPI() *SchemaBuilder {
	cp := b.Clone()
	cp.hideCoreAPI = true
	return cp
}

// Prepend returns a new SchemaBuilder with the given modules (default opts)
// inserted before the existing entries.
func (b *SchemaBuilder) Prepend(mods ...Mod) *SchemaBuilder {
	extra := make([]modDepEntry, len(mods))
	for i, m := range mods {
		extra[i] = modDepEntry{mod: m}
	}
	return &SchemaBuilder{
		root:        b.root,
		entries:     append(extra, b.entries...),
		hideCoreAPI: b.hideCoreAPI,
	}
}

// Append returns a new SchemaBuilder with the given modules (default opts)
// appended after the existing entries.
func (b *SchemaBuilder) Append(mods ...Mod) *SchemaBuilder {
	extra := make([]modDepEntry, len(mods))
	for i, m := range mods {
		extra[i] = modDepEntry{mod: m}
	}
	return &SchemaBuilder{
		root:        b.root,
		entries:     append(slices.Clone(b.entries), extra...),
		hideCoreAPI: b.hideCoreAPI,
	}
}

// With returns a new SchemaBuilder that includes the given module with
// the specified install options. If the module is already present (by
// name), it is not duplicated — but its options are promoted to the less
// restrictive combination of old and new.
func (b *SchemaBuilder) With(mod Mod, opts InstallOpts) *SchemaBuilder {
	cp := &SchemaBuilder{
		root:        b.root,
		entries:     slices.Clone(b.entries),
		hideCoreAPI: b.hideCoreAPI,
	}
	for i, e := range cp.entries {
		if e.mod.Name() == mod.Name() {
			promoted := e.opts
			if promoted.SkipConstructor && !opts.SkipConstructor {
				promoted.SkipConstructor = false
			}
			if !promoted.Entrypoint && opts.Entrypoint {
				promoted.Entrypoint = true
			}
			cp.entries[i].opts = promoted
			return cp
		}
	}
	cp.entries = append(cp.entries, modDepEntry{mod: mod, opts: opts})
	return cp
}

// Lookup returns the module with the given name, if present.
func (b *SchemaBuilder) Lookup(name string) (Mod, bool) {
	for _, e := range b.entries {
		if e.mod.Name() == name {
			return e.mod, true
		}
	}
	return nil, false
}

// Mods returns the list of all modules (regardless of install policy).
func (b *SchemaBuilder) Mods() []Mod {
	mods := make([]Mod, len(b.entries))
	for i, e := range b.entries {
		mods[i] = e.mod
	}
	return mods
}

// PrimaryMods returns only the modules whose constructors should appear
// on the Query root (i.e. those not installed with SkipConstructor).
func (b *SchemaBuilder) PrimaryMods() []Mod {
	var mods []Mod
	for _, e := range b.entries {
		if !e.opts.SkipConstructor {
			mods = append(mods, e.mod)
		}
	}
	return mods
}

// Server builds and caches the dagql server for all modules. When any
// module has Entrypoint set, entrypoint proxy fields are installed on
// Query, and ID loading is delegated to an inner server without proxies
// so that IDs are always evaluated against a clean schema.
func (b *SchemaBuilder) Server(ctx context.Context) (*dagql.Server, error) {
	srv, err := b.lazilyLoadSchema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	dagqlCache, err := b.root.Cache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache: %w", err)
	}
	return srv.WithCache(dagqlCache), nil
}

// SchemaIntrospectionJSONFile returns an introspection JSON file for the
// schema, optionally hiding the given types. The dagql Select cache
// (CachePerSchema) handles caching per-args, so different hiddenTypes
// produce correctly different results.
func (b *SchemaBuilder) SchemaIntrospectionJSONFile(ctx context.Context, hiddenTypes []string) (dagql.Result[*File], error) {
	dag, err := b.Server(ctx)
	if err != nil {
		return dagql.Result[*File]{}, err
	}
	return schemaJSONFileFromServer(ctx, dag, hiddenTypes)
}

// SchemaIntrospectionJSONFileForModule returns an introspection JSON file
// with types hidden that should not be exposed to module SDKs.
func (b *SchemaBuilder) SchemaIntrospectionJSONFileForModule(ctx context.Context) (dagql.Result[*File], error) {
	// Include both the module-specific hidden types and the engine-internal types
	hiddenTypes := append([]string{}, TypesToIgnoreForModuleIntrospection...)
	for _, typed := range TypesHiddenFromModuleSDKs {
		hiddenTypes = append(hiddenTypes, typed.Type().Name())
	}
	return b.SchemaIntrospectionJSONFile(ctx, hiddenTypes)
}

// SchemaIntrospectionJSONFileForClient returns an introspection JSON file
// for standalone client generation. Unlike module SDKs, standalone clients
// have access to Engine and other types that are hidden from modules.
func (b *SchemaBuilder) SchemaIntrospectionJSONFileForClient(ctx context.Context) (dagql.Result[*File], error) {
	return b.SchemaIntrospectionJSONFile(ctx, []string{})
}

// TypeDefs returns type definitions for all modules by introspecting the
// combined schema. Directives in the schema carry module metadata
// (SourceModuleName, defaultPath, etc.), so no merging step is required.
func (b *SchemaBuilder) TypeDefs(ctx context.Context, dag *dagql.Server) ([]*TypeDef, error) {
	return TypeDefsFromSchema(dag, nil)
}

// ModTypeFor searches the modules for the given type def, returning the
// ModType if found. This does not recurse to transitive dependencies; it
// only returns types directly exposed by the schema of the top-level deps.
func (b *SchemaBuilder) ModTypeFor(ctx context.Context, typeDef *TypeDef) (ModType, bool, error) {
	for _, e := range b.entries {
		modType, ok, err := e.mod.ModTypeFor(ctx, typeDef, false)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get type from mod %q: %w", e.mod.Name(), err)
		}
		if ok {
			return modType, true, nil
		}
	}
	return nil, false, nil
}

func (b *SchemaBuilder) lazilyLoadSchema(ctx context.Context) (
	loadedSchema *dagql.Server,
	rerr error,
) {
	b.loadSchemaLock.Lock()
	defer b.loadSchemaLock.Unlock()
	if b.lazilyLoadedServer != nil {
		return b.lazilyLoadedServer, nil
	}
	if b.loadSchemaErr != nil {
		return nil, b.loadSchemaErr
	}
	defer func() {
		b.lazilyLoadedServer = loadedSchema
		b.loadSchemaErr = rerr
	}()

	// Check if any entry has Entrypoint set.
	var nonEntrypoints, entrypoints []modDepEntry
	for _, e := range b.entries {
		if e.opts.Entrypoint {
			entrypoints = append(entrypoints, e)
		} else {
			nonEntrypoints = append(nonEntrypoints, e)
		}
	}

	needsSplit := len(entrypoints) > 0 || b.hideCoreAPI

	if !needsSplit {
		// No entrypoints and no core API hiding — single server suffices.
		mods := make([]modInstall, len(b.entries))
		for i, e := range b.entries {
			mods[i] = modInstall(e)
		}
		dag, err := buildSchema(ctx, b.root, mods)
		if err != nil {
			return nil, err
		}
		return dag, nil
	}

	// Build inner server: all modules with Entrypoint forced to false.
	// This is the canonical server used for ID loading and proxy resolution.
	innerMods := make([]modInstall, len(b.entries))
	for i, e := range b.entries {
		opts := e.opts
		opts.Entrypoint = false
		innerMods[i] = modInstall{mod: e.mod, opts: opts}
	}
	inner, err := buildSchema(ctx, b.root, innerMods)
	if err != nil {
		return nil, err
	}

	// Build outer server: all modules with real Entrypoint flags.
	outerMods := make([]modInstall, 0, len(b.entries))
	for _, e := range nonEntrypoints {
		outerMods = append(outerMods, modInstall(e))
	}
	for _, e := range entrypoints {
		outerMods = append(outerMods, modInstall(e))
	}
	outer, err := buildSchema(ctx, b.root, outerMods)
	if err != nil {
		return nil, err
	}

	// When HideCoreAPI is set, strip core Query-root fields from the
	// outer server so clients only see module entrypoints and
	// bootstrapping primitives. Core types (Container, Directory, etc.)
	// remain in the schema for return-type resolution.
	if b.hideCoreAPI {
		stripCoreQueryFields(outer)
	}

	// Wire up delegation: the outer server's Load, LoadType, and
	// Canonical() all route to the inner server, ensuring IDs are
	// canonical and proxy resolvers can reach the real constructors.
	outer.SetCanonical(inner)

	return outer, nil
}

// bootstrapQueryFields are Query-root fields that must remain on the
// client-facing server even when HideCoreAPI is set, because the CLI
// and other tooling depend on them for schema discovery.
var bootstrapQueryFields = map[string]bool{
	"currentTypeDefs":      true,
	"currentModule":        true,
	"currentFunctionCall":  true,
	"version":              true,
}

// stripCoreQueryFields removes core-originated fields from the outer
// server's Query root, keeping only module-provided fields and a small
// set of bootstrapping primitives.
func stripCoreQueryFields(dag *dagql.Server) {
	queryType, ok := dag.ObjectType("Query")
	if !ok {
		return
	}
	queryType.StripFields(func(name string, spec dagql.FieldSpec) bool {
		// Keep fields installed by modules (constructors, entrypoint proxies, etc.)
		if spec.Module != nil {
			return true
		}
		// Keep bootstrapping fields needed by the CLI.
		if bootstrapQueryFields[name] {
			return true
		}
		return false
	})
}
