package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dagger/dagger/core"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

// CoreMod is a special implementation of Mod for our core API, which is not *technically* a true module yet
// but can be treated as one in terms of dependencies. It has no dependencies itself and is currently an
// implicit dependency of every user module.
type CoreMod struct {
	dag               *dagql.Server
	introspectionJSON json.RawMessage
}

var _ core.Mod = (*CoreMod)(nil)

func (m *CoreMod) Name() string {
	return core.ModuleName
}

func (m *CoreMod) Dependencies() []core.Mod {
	return nil
}

func (m *CoreMod) Install(ctx context.Context, dag *dagql.Server) error {
	for _, schema := range []SchemaResolvers{
		&querySchema{dag},
		&directorySchema{dag},
		&fileSchema{dag},
		&gitSchema{dag},
		&containerSchema{dag},
		&cacheSchema{dag},
		&secretSchema{dag},
		&serviceSchema{dag},
		&hostSchema{dag},
		&httpSchema{dag},
		&platformSchema{dag},
		&socketSchema{dag},
		&moduleSchema{dag},
	} {
		schema.Install()
	}
	return nil
}

func (m *CoreMod) SchemaIntrospectionJSON(_ context.Context, _ *core.Query) (string, error) {
	return string(m.introspectionJSON), nil
}

func (m *CoreMod) ModTypeFor(ctx context.Context, typeDef *core.TypeDef, checkDirectDeps bool) (core.ModType, bool, error) {
	var modType core.ModType

	switch typeDef.Kind {
	case core.TypeDefKindString, core.TypeDefKindInteger, core.TypeDefKindBoolean, core.TypeDefKindVoid:
		modType = &core.PrimitiveType{Def: typeDef}

	case core.TypeDefKindList:
		underlyingType, ok, err := m.ModTypeFor(ctx, typeDef.AsList.Value.ElementTypeDef, checkDirectDeps)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get underlying type: %w", err)
		}
		if !ok {
			return nil, false, nil
		}
		modType = &core.ListType{
			Elem:       typeDef.AsList.Value.ElementTypeDef,
			Underlying: underlyingType,
		}

	case core.TypeDefKindObject:
		// TODO
		_, ok := m.dag.ObjectType(typeDef.AsObject.Value.Name)
		if !ok {
			log.Println("!!!!!! CORE DID NOT FIND OBJECT TYPE", typeDef.AsObject.Value.Name)
			return nil, false, nil
		}
		log.Println("!!! CORE FOUND OBJECT TYPE", typeDef.AsObject.Value.Name)
		modType = &CoreModObject{coreMod: m}

	default:
		return nil, false, fmt.Errorf("unexpected type def kind %s", typeDef.Kind)
	}

	if typeDef.Optional {
		modType = &core.NullableType{
			Elem:       typeDef.WithOptional(false),
			Underlying: modType,
		}
	}

	return modType, true, nil
}

// CoreModObject represents objects from core (Container, Directory, etc.)
type CoreModObject struct {
	coreMod *CoreMod
}

var _ core.ModType = (*CoreModObject)(nil)

func (obj *CoreModObject) ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error) {
	if value == nil {
		return nil, nil
	}
	id, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", value)
	}
	var idp idproto.ID
	if err := idp.Decode(id); err != nil {
		return nil, err
	}
	val, err := obj.coreMod.dag.Load(ctx, &idp)
	if err != nil {
		return nil, fmt.Errorf("CoreModObject.load %s: %w", idp.Display(), err)
	}
	return val, nil
}

func (obj *CoreModObject) ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch x := value.(type) {
	case dagql.Input:
		return x, nil
	case dagql.Object:
		return x.ID().Encode()
	default:
		return nil, fmt.Errorf("CoreModObject.ConvertToSDKInput: unknown type %T", value)
	}
}

func (obj *CoreModObject) SourceMod() core.Mod {
	return obj.coreMod
}
