package core

import (
	"context"
	"fmt"

	"github.com/vito/dagql"
)

// ModType wraps the core TypeDef type with schema specific concerns like ID conversion
// and tracking of the module in which the type was originally defined.
type ModType interface {
	// ConvertFromSDKResult converts a value returned from an SDK into values
	// expected by the server, including conversion of IDs to their "unpacked"
	// objects
	ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error)

	// ConvertToSDKInput converts a value from the server into a value expected
	// by the SDK, which may include converting objects to their IDs
	ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error)

	// SourceMod is the module in which this type was originally defined
	SourceMod() Mod

	// The core API TypeDef representation of this type
	TypeDef() *TypeDef
}

// PrimitiveType are the basic types like string, int, bool, void, etc.
type PrimitiveType struct {
	Def *TypeDef
}

func (t *PrimitiveType) ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error) {
	// NB: we lean on the fact that all primitive types are also dagql.Inputs
	return t.Def.ToInput().Decoder().DecodeInput(value)
}

func (t *PrimitiveType) ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error) {
	return value, nil
}

func (t *PrimitiveType) SourceMod() Mod {
	return nil
}

func (t *PrimitiveType) TypeDef() *TypeDef {
	return t.Def
}

type ListType struct {
	Elem       *TypeDef
	Underlying ModType
}

func (t *ListType) ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error) {
	if value == nil {
		return nil, nil
	}

	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("ListType.ConvertFromSDKResult: expected []any, got %T", value)
	}
	resultList := make([]dagql.Typed, len(list))
	for i, item := range list {
		var err error
		resultList[i], err = t.Underlying.ConvertFromSDKResult(ctx, item)
		if err != nil {
			return nil, err
		}
	}
	return dagql.DynamicArrayOutput{
		Elem:   t.Elem.ToTyped(),
		Values: resultList,
	}, nil
}

func (t *ListType) ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error) {
	if value == nil {
		return nil, nil
	}
	list, ok := value.(dagql.DynamicArrayInput)
	if !ok {
		return nil, fmt.Errorf("ListType.ConvertToSDKInput: expected DynamicArrayInput, got %T: %#v", value, value)
	}
	resultList := make([]any, len(list.Values))
	for i, item := range list.Values {
		var err error
		resultList[i], err = t.Underlying.ConvertToSDKInput(ctx, item)
		if err != nil {
			return nil, err
		}
	}
	return resultList, nil
}

func (t *ListType) SourceMod() Mod {
	return t.Underlying.SourceMod()
}

func (t *ListType) TypeDef() *TypeDef {
	return &TypeDef{
		Kind: TypeDefKindList,
		AsList: dagql.NonNull(&ListTypeDef{
			ElementTypeDef: t.Elem.Clone(),
		}),
	}
}

type ModuleObjectType struct {
	typeDef *ObjectTypeDef
	mod     *Module
}

func (t *ModuleObjectType) SourceMod() Mod {
	return t.mod
}

func (obj *ModuleObjectType) ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error) {
	if value == nil {
		return nil, nil
	}

	switch value := value.(type) {
	case map[string]any:
		return &ModuleObject{
			TypeDef: obj.typeDef,
			Fields:  value,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected result value type %T for object %q", value, obj.typeDef.Name)
	}
}

func (obj *ModuleObjectType) ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error) {
	if value == nil {
		return nil, nil
	}
	// NOTE: user mod objects are currently only passed as inputs to the module
	// they originate from; modules can't have inputs/outputs from other modules
	// (other than core). These objects are also passed as their direct json
	// serialization rather than as an ID (so that SDKs can decode them without
	// needing to make calls to their own API).
	switch x := value.(type) {
	case DynamicID:
		dag, err := obj.mod.Deps.Prepend(obj.mod).Schema(ctx) // TODO: this seems expensive
		if err != nil {
			return nil, fmt.Errorf("schema: %w", err)
		}
		val, err := dag.Load(ctx, x.ID())
		if err != nil {
			return nil, fmt.Errorf("load DynamicID: %w", err)
		}
		switch x := val.(type) {
		case dagql.Instance[*ModuleObject]:
			return x.Self.Fields, nil
		default:
			return nil, fmt.Errorf("unexpected value type %T", x)
		}
	default:
		return nil, fmt.Errorf("ModuleObjectType.ConvertToSDKInput cannot handle %T", x)
	}
}

func (t *ModuleObjectType) TypeDef() *TypeDef {
	return &TypeDef{
		Kind:     TypeDefKindObject,
		AsObject: dagql.NonNull(t.typeDef),
	}
}

type NullableType struct {
	Elem       *TypeDef
	Underlying ModType
}

func (t *NullableType) ConvertFromSDKResult(ctx context.Context, value any) (dagql.Typed, error) {
	if value == nil {
		return nil, nil
	}
	val, err := t.Underlying.ConvertFromSDKResult(ctx, value)
	if err != nil {
		return nil, err
	}
	return dagql.DynamicNullable{
		Elem:  t.Elem.ToTyped(),
		Value: val,
		Valid: true,
	}, nil
}

func (t *NullableType) ConvertToSDKInput(ctx context.Context, value dagql.Typed) (any, error) {
	if value == nil {
		return nil, nil
	}
	opt, ok := value.(dagql.DynamicOptional)
	if !ok {
		return nil, fmt.Errorf("NullableType.ConvertToSDKInput: expected DynamicArrayInput, got %T: %#v", value, value)
	}
	if !opt.Valid {
		return nil, nil
	}
	result, err := t.Underlying.ConvertToSDKInput(ctx, opt.Value)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (t *NullableType) SourceMod() Mod {
	return t.Underlying.SourceMod()
}

func (t *NullableType) TypeDef() *TypeDef {
	cp := t.Elem.Clone()
	cp.Optional = true
	return cp
}
