package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/moby/buildkit/util/bklog"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

// ModuleObject is an object defined by a user module
type ModuleObject struct {
	mod   *Module
	modID *idproto.ID

	// the type def metadata, with namespacing already applied
	typeDef *TypeDef

	// should not be read directly, call Fields() and Functions() instead
	lazyLoadedFields    []*UserModField
	lazyLoadedFunctions []*ModuleFunction
	loadErr             error
	loadLock            sync.Mutex
}

func newModObject(mod *Module, modID *idproto.ID, typeDef *TypeDef) (*ModuleObject, error) {
	if typeDef.Kind != TypeDefKindObject {
		return nil, fmt.Errorf("expected object type def, got %s", typeDef.Kind)
	}
	obj := &ModuleObject{
		mod:     mod,
		modID:   modID,
		typeDef: typeDef,
	}
	return obj, nil
}

func (obj *ModuleObject) TypeDef() *TypeDef {
	return obj.typeDef
}

var _ dagql.ObjectType = (*ModuleObject)(nil)

func (obj *ModuleObject) Extend(dagql.FieldSpec, dagql.FieldFunc) {
	panic("not implemented")
}

func (obj *ModuleObject) SourceMod() Mod {
	return obj.mod
}

func (obj *ModuleObject) loadFieldsAndFunctions(ctx context.Context) (
	loadedFields []*UserModField, loadedFunctions []*ModuleFunction, rerr error,
) {
	obj.loadLock.Lock()
	defer obj.loadLock.Unlock()
	if len(obj.lazyLoadedFields) > 0 || len(obj.lazyLoadedFunctions) > 0 {
		return obj.lazyLoadedFields, obj.lazyLoadedFunctions, nil
	}
	if obj.loadErr != nil {
		return nil, nil, obj.loadErr
	}
	defer func() {
		obj.lazyLoadedFields = loadedFields
		obj.lazyLoadedFunctions = loadedFunctions
		obj.loadErr = rerr
	}()

	mod := obj.mod

	for _, fieldTypeDef := range obj.typeDef.AsObject.Value.Fields {
		modField, err := newModField(ctx, obj, fieldTypeDef)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create field: %w", err)
		}
		loadedFields = append(loadedFields, modField)
	}
	for _, fn := range obj.typeDef.AsObject.Value.Functions {
		modFunction, err := newModFunction(ctx, mod.Query, obj.mod, obj.modID, obj, mod.Runtime, fn)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create function: %w", err)
		}
		loadedFunctions = append(loadedFunctions, modFunction)
	}
	return loadedFields, loadedFunctions, nil
}

func (obj *ModuleObject) FieldByName(ctx context.Context, name string) (*UserModField, bool, error) {
	fields, _, err := obj.loadFieldsAndFunctions(ctx)
	if err != nil {
		return nil, false, err
	}

	name = gqlFieldName(name)
	for _, f := range fields {
		if gqlFieldName(f.metadata.Name) == name {
			return f, true, nil
		}
	}

	return nil, false, nil
}

func (obj *ModuleObject) FunctionByName(ctx context.Context, name string) (*ModuleFunction, bool, error) {
	_, functions, err := obj.loadFieldsAndFunctions(ctx)
	if err != nil {
		return nil, false, err
	}

	name = gqlFieldName(name)
	for _, fn := range functions {
		if gqlFieldName(fn.metadata.Name) == name {
			return fn, true, nil
		}
	}

	return nil, false, nil
}

func (obj *ModuleObject) Type() *ast.Type {
	return &ast.Type{
		NamedType: obj.typeDef.AsObject.Value.Name,
		NonNull:   true,
	}
}

func (obj *ModuleObject) TypeName() string {
	return obj.typeDef.AsObject.Value.Name
}

func (obj *ModuleObject) New(id *idproto.ID, t dagql.Typed) (dagql.Object, error) {
	switch x := t.(type) {
	// case JSON: // not sure, maybe this is map[string]any instead?
	// 	return &UserModInstance{
	// 		id:  id,
	// 		typ: t,
	// 		obj: obj,
	// 	}, nil
	case *DynamicObject:
		return &UserModInstance{
			id:  id,
			typ: t,
			obj: obj,
			val: x.Fields,
		}, nil
	default:
		return nil, fmt.Errorf("UserModObject.New: unexpected type %T", t)
	}
}

func (obj *ModuleObject) Definition() *ast.Definition {
	def := obj.typeDef.AsObject.Value
	astDef := &ast.Definition{
		Kind:        ast.Object,
		Name:        def.Name,
		Description: formatGqlDescription(def.Description),
	}
	astDef.Fields = append(astDef.Fields, &ast.FieldDefinition{
		Name:        "id",
		Description: "TODO ID description",
		Type:        DynamicID{Object: *obj.typeDef.AsObject.Value}.Type(),
	})
	for _, field := range def.Fields {
		astDef.Fields = append(astDef.Fields, &ast.FieldDefinition{
			Name:        field.Name,
			Description: formatGqlDescription(field.Description),
			Type:        field.TypeDef.ToType(),
		})
	}
	for _, fun := range def.Functions {
		fieldDef := &ast.FieldDefinition{
			Name:        fun.Name,
			Description: formatGqlDescription(fun.Description),
			Type:        fun.ReturnType.ToType(),
		}
		for _, arg := range fun.Args {
			fieldDef.Arguments = append(fieldDef.Arguments, &ast.ArgumentDefinition{
				Name:         arg.Name,
				Description:  formatGqlDescription(arg.Description),
				Type:         arg.TypeDef.ToInput().Type(),
				DefaultValue: arg.DefaultValue.ToLiteral().ToAST(),
			})
		}
		astDef.Fields = append(astDef.Fields, fieldDef)
	}
	return astDef
}

// ParseField parses a field selection into a Selector and return type.
func (obj *ModuleObject) ParseField(ctx context.Context, astField *ast.Field, vars map[string]any) (dagql.Selector, *ast.Type, error) {
	if astField.Name == "id" {
		if len(astField.Arguments) > 0 {
			return dagql.Selector{}, nil, fmt.Errorf("id is a field; it doesn't take arguments")
		}
		return dagql.Selector{
			Field: astField.Name,
		}, DynamicID{Object: *obj.TypeDef().AsObject.Value}.Type(), nil
	}

	field, isField, err := obj.FieldByName(ctx, astField.Name)
	if err != nil {
		// this would be an error loading the fields
		return dagql.Selector{}, nil, err
	}
	if isField {
		def := field.metadata.TypeDef
		if len(astField.Arguments) > 0 {
			return dagql.Selector{}, nil, fmt.Errorf("%q is a field; it doesn't take arguments", astField.Name)
		}
		return dagql.Selector{
			Field: astField.Name,
		}, def.ToType(), nil
	}
	fun, isFun, err := obj.FunctionByName(ctx, astField.Name)
	if err != nil {
		// this would be an error loading the fields
		return dagql.Selector{}, nil, err
	}
	if !isFun {
		return dagql.Selector{}, nil, fmt.Errorf("Cannot query field %q of object %q", astField.Name, obj.Definition().Name)
	}
	def := fun.metadata
	args := make([]dagql.NamedInput, len(astField.Arguments))
	for i, arg := range astField.Arguments {
		argSpec, ok := def.LookupArg(arg.Name)
		if !ok {
			return dagql.Selector{}, nil, fmt.Errorf("%s.%s has no such argument: %q", obj.TypeName(), astField.Name, arg.Name)
		}
		val, err := arg.Value.Value(vars)
		if err != nil {
			return dagql.Selector{}, nil, err
		}
		if val == nil {
			continue
		}
		input, err := argSpec.TypeDef.ToInput().Decoder().DecodeInput(val)
		if err != nil {
			return dagql.Selector{}, nil, fmt.Errorf("init arg %q value: %w", arg.Name, err)
		}
		args[i] = dagql.NamedInput{
			Name:  arg.Name,
			Value: input,
		}
	}
	return dagql.Selector{
		Field: astField.Name,
		Args:  args,
	}, def.ReturnType.ToType(), nil
}

type UserModInstance struct {
	id  *idproto.ID
	typ dagql.Typed
	obj *ModuleObject
	val map[string]any
}

var _ dagql.Object = (*UserModInstance)(nil)

func (inst *UserModInstance) ObjectType() dagql.ObjectType {
	return inst.obj
}

func (inst *UserModInstance) ID() *idproto.ID {
	return inst.id
}

func (inst *UserModInstance) Type() *ast.Type {
	return &ast.Type{
		NamedType: inst.obj.typeDef.AsObject.Value.Name,
		NonNull:   true,
	}
}

func (inst *UserModInstance) IDFor(ctx context.Context, sel dagql.Selector) (*idproto.ID, error) {
	if sel.Field == "id" {
		return inst.id.Append(
			DynamicID{Object: *inst.obj.TypeDef().AsObject.Value}.Type(),
			"id",
		), nil
	}

	field, found, err := inst.obj.FieldByName(ctx, sel.Field)
	if err != nil {
		return nil, err
	}
	if found {
		return sel.AppendTo(
			inst.id,
			field.metadata.TypeDef.ToType(),
			// TODO: for now all object functions are tainted, meaning they won't
			// be cached.
			true,
		), nil
	}
	fun, found, err := inst.obj.FunctionByName(ctx, sel.Field)
	if found {
		return sel.AppendTo(
			inst.id,
			fun.metadata.ReturnType.ToType(),
			// TODO: for now all object functions are tainted, meaning they won't
			// be cached.
			true,
		), nil
	}
	return nil, fmt.Errorf("field %q not found on object %q", sel.Field, inst.obj.typeDef.AsObject.Value.Name)
}

func (inst *UserModInstance) Select(ctx context.Context, sel dagql.Selector) (dagql.Typed, error) {
	switch sel.Field {
	case "id":
		return DynamicID{
			Object: *inst.obj.typeDef.AsObject.Value,
			ID:     inst.id,
		}, nil
	default:
		val, err := inst.selectInner(ctx, sel)
		if err != nil {
			return nil, err
		}
		// FIXME(vito): the following boilerplate is copied from DagQL's Instance
		// implementation. a bit a of a smell that this needs to be replicated
		// here, but it's also kind of unusual to have to implement a custom
		// Instance in the first place. maybe move to a helper in DagQL?
		if n, ok := val.(dagql.NullableWrapper); ok {
			val, ok = n.Unwrap()
			if !ok {
				return nil, nil
			}
		}
		if sel.Nth != 0 {
			enum, ok := val.(dagql.Enumerable)
			if !ok {
				return nil, fmt.Errorf("cannot sub-select %dth item from %T", sel.Nth, val)
			}
			val, err = enum.Nth(sel.Nth)
			if err != nil {
				return nil, err
			}
			if n, ok := val.(dagql.NullableWrapper); ok {
				val, ok = n.Unwrap()
				if !ok {
					return nil, nil
				}
			}
		}
		return val, nil
	}
}

func (inst *UserModInstance) selectInner(ctx context.Context, sel dagql.Selector) (dagql.Typed, error) {
	fun, found, err := inst.obj.FunctionByName(ctx, sel.Field)
	if err != nil {
		return nil, err
	}
	if found {
		opts := &CallOpts{
			ParentVal: inst.val,
			Cache:     false, // TODO
			// Pipeline:  _, // TODO
			SkipSelfSchema: false, // TODO?
		}
		for _, arg := range sel.Args {
			opts.Inputs = append(opts.Inputs, CallInput{
				Name:  arg.Name,
				Value: arg.Value,
			})
		}
		return fun.Call(ctx, dagql.CurrentID(ctx), opts)
	}
	field, found, err := inst.obj.FieldByName(ctx, sel.Field)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field %q not found on object %q", sel.Field, inst.obj.typeDef.AsObject.Value.Name)
	}
	fieldVal, found := inst.val[field.metadata.OriginalName]
	if !found {
		return nil, fmt.Errorf("field %q not found on object %q", sel.Field, inst.obj.typeDef.AsObject.Value.Name)
	}
	return field.modType.ConvertFromSDKResult(ctx, fieldVal)
}

func (obj *ModuleObject) Install(ctx context.Context, dag *dagql.Server) error {
	ctx = bklog.WithLogger(ctx, bklog.G(ctx).WithField("object", obj.typeDef.AsObject.Value.Name))
	bklog.G(ctx).Debug("getting object schema")

	dag.InstallObject(obj)

	objTypeDef := obj.typeDef.AsObject.Value
	objName := objTypeDef.Name

	mod := obj.mod

	// check whether this is a pre-existing object from a dependency module
	modType, ok, err := mod.Deps.ModTypeFor(ctx, obj.typeDef)
	if err != nil {
		return fmt.Errorf("failed to get mod type for type def: %w", err)
	}
	if ok {
		log.Println("!!! GOT MOD TYPE", obj.typeDef.ToType(), fmt.Sprintf("%T", modType))
		if sourceMod := modType.SourceMod(); sourceMod != nil && sourceMod != mod {
			// modules can reference types from core/other modules as types, but they
			// can't attach any new fields or functions to them
			if len(objTypeDef.Fields) > 0 || len(objTypeDef.Functions) > 0 {
				return fmt.Errorf("cannot attach new fields or functions to object %q from outside module", objName)
			}
			return nil
		}
	}

	log.Println("!!! INSTALLING OBJECT", objName)

	dag.InstallScalar(DynamicID{
		Object: *obj.TypeDef().AsObject.Value,
	})

	// handle object constructor
	isMainModuleObject := objName == gqlObjectName(mod.Name())
	if isMainModuleObject {
		if objTypeDef.Constructor.Valid {
			// use explicit user-defined constructor if provided
			fnTypeDef := objTypeDef.Constructor.Value
			if fnTypeDef.ReturnType.Kind != TypeDefKindObject {
				return fmt.Errorf("constructor function for object %s must return that object", objTypeDef.OriginalName)
			}
			if fnTypeDef.ReturnType.AsObject.Value.OriginalName != objTypeDef.OriginalName {
				return fmt.Errorf("constructor function for object %s must return that object", objTypeDef.OriginalName)
			}

			fn, err := newModFunction(ctx, mod.Query, obj.mod, obj.modID, obj, mod.Runtime, fnTypeDef)
			if err != nil {
				return fmt.Errorf("failed to create function: %w", err)
			}

			spec := dagql.FieldSpec{
				Name:        gqlFieldName(mod.Name()),
				Description: formatGqlDescription(fn.metadata.Description),
				Type:        fn.metadata.ReturnType.ToTyped(),
				Pure:        true,
			}

			for _, arg := range fnTypeDef.Args {
				input := arg.TypeDef.ToInput()
				var defaultVal dagql.Input
				if arg.DefaultValue != nil {
					var val any
					dec := json.NewDecoder(bytes.NewReader(arg.DefaultValue.Bytes()))
					dec.UseNumber()
					if err := dec.Decode(&val); err != nil {
						return fmt.Errorf("failed to decode default value for arg %q: %w", arg.Name, err)
					}
					defaultVal, err = input.Decoder().DecodeInput(val)
					if err != nil {
						return fmt.Errorf("failed to decode default value for arg %q: %w", arg.Name, err)
					}
				}
				spec.Args = append(spec.Args, dagql.InputSpec{
					Name:        arg.Name,
					Description: formatGqlDescription(arg.Description),
					Type:        input,
					Default:     defaultVal,
				})
			}

			dag.Root().ObjectType().Extend(
				spec,
				func(ctx context.Context, self dagql.Object, args map[string]dagql.Typed) (dagql.Typed, error) {
					var callInput []CallInput
					for k, v := range args {
						callInput = append(callInput, CallInput{
							Name:  k,
							Value: v,
						})
					}
					return fn.Call(ctx, dagql.CurrentID(ctx), &CallOpts{
						Inputs:    callInput,
						ParentVal: nil,
					})
				},
			)
		} else {
			// otherwise default to a simple field with no args that returns an initially empty object
			dag.Root().ObjectType().Extend(
				dagql.FieldSpec{
					Name:        gqlFieldName(mod.Name()),
					Description: "TODO",
					Type:        obj,
					Pure:        true,
				},
				func(ctx context.Context, self dagql.Object, _ map[string]dagql.Typed) (dagql.Typed, error) {
					return &DynamicObject{
						Object: *obj.typeDef.AsObject.Value,
						Fields: map[string]any{},
					}, nil
				},
			)
		}
	}

	dag.Root().ObjectType().Extend(
		dagql.FieldSpec{
			Name: fmt.Sprintf("load%sFromID", objName),
			Type: obj,
			Pure: false, // no need to cache this; what if the ID is impure?
			Args: []dagql.InputSpec{
				{
					Name: "id",
					Type: DynamicID{Object: *obj.typeDef.AsObject.Value},
				},
			},
		},
		func(ctx context.Context, self dagql.Object, args map[string]dagql.Typed) (dagql.Typed, error) {
			log.Println("!!! LOADING OBJECT FROM ID", args["id"].(DynamicID).ID.Display())
			return dag.Load(ctx, args["id"].(DynamicID).ID)
		},
	)

	log.Println("!!! INSTALLED OBJECT", objName)

	return nil
}

type UserModField struct {
	obj      *ModuleObject
	metadata *FieldTypeDef
	modType  ModType
}

func newModField(ctx context.Context, obj *ModuleObject, metadata *FieldTypeDef) (*UserModField, error) {
	modType, ok, err := obj.mod.ModTypeFor(ctx, metadata.TypeDef, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get mod type for field %q: %w", metadata.Name, err)
	}
	if !ok {
		return nil, fmt.Errorf("failed to get mod type for field %q", metadata.Name)
	}
	return &UserModField{
		obj:      obj,
		metadata: metadata,
		modType:  modType,
	}, nil
}

/*
This formats comments in the schema as:
"""
comment
"""

Which avoids corner cases where the comment ends in a `"`.
*/
func formatGqlDescription(desc string, args ...any) string {
	if desc == "" {
		return ""
	}
	return "\n" + strings.TrimSpace(fmt.Sprintf(desc, args...)) + "\n"
}

func gqlObjectName(name string) string {
	// gql object name is capitalized camel case
	return strcase.ToCamel(name)
}

func namespaceObject(objName, namespace string) string {
	gqlObjName := gqlObjectName(objName)
	if rest := strings.TrimPrefix(gqlObjName, gqlObjectName(namespace)); rest != gqlObjName {
		if len(rest) == 0 {
			// objName equals namespace, don't namespace this
			return gqlObjName
		}
		// we have this case check here to check for a boundary
		// e.g. if objName="Postman" and namespace="Post", then we should still namespace
		// this to "PostPostman" instead of just going for "Postman" (but we should do that
		// if objName="PostMan")
		if 'A' <= rest[0] && rest[0] <= 'Z' {
			// objName has namespace prefixed, don't namespace this
			return gqlObjName
		}
	}

	return gqlObjectName(namespace + "_" + objName)
}

func gqlFieldName(name string) string {
	// gql field name is uncapitalized camel case
	return strcase.ToLowerCamel(name)
}

func gqlArgName(name string) string {
	// gql arg name is uncapitalized camel case
	return strcase.ToLowerCamel(name)
}
