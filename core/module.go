package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dagger/dagger/core/modules"
	"github.com/moby/buildkit/solver/pb"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

type Module struct {
	Query *Query

	// The module's source code root directory
	SourceDirectory dagql.Instance[*Directory] `field:"true"`

	// If set, the subdir of the SourceDirectory that contains the module's source code
	SourceDirectorySubpath string `field:"true"`

	// The name of the module
	NameField string `name:"name" field:"true"`

	// The doc string of the module, if any
	Description *string `field:"true"`

	// The module's SDKConfig, as set in the module config file
	SDKConfig string `name:"sdk" field:"true"`

	// Dependencies as configured by the module
	DependencyConfig []string `field:"true"`

	// The module's loaded dependencies.
	DependenciesField []dagql.Instance[*Module] `name:"dependencies" field:"true"`

	// The following are populated while initializing the module
	Deps *ModDeps

	// GeneratedCode is the generated code for the module, which is available
	// even if it doesn't properly compile.
	GeneratedCode *GeneratedCode `field:"true"`

	// Runtime is the container that runs the module's entrypoint. It is
	// unavailable if the module doesn't compile.
	Runtime *Container

	// InstanceID is the ID of the initialized module.
	InstanceID *idproto.ID

	// The module's objects
	ObjectDefs []*TypeDef `name:"objects" field:"true"`

	// The module's interfaces
	InterfaceDefs []*TypeDef `json:"interfaces,omitempty"`
}

var _ Mod = (*Module)(nil)

func (mod *Module) Name() string {
	return mod.NameField
}

func (mod *Module) Dependencies() []Mod {
	mods := make([]Mod, len(mod.DependenciesField))
	for i, dep := range mod.DependenciesField {
		mods[i] = dep.Self
	}
	return mods
}

func (mod *Module) Initialize(ctx context.Context, oldSelf dagql.Instance[*Module], newID *idproto.ID) (*Module, error) {
	// construct a special function with no object or function name, which tells
	// the SDK to return the module's definition (in terms of objects, fields and
	// functions)
	getModDefFn, err := newModFunction(
		ctx,
		mod.Query,
		oldSelf.Self,
		oldSelf.ID(),
		nil,
		mod.Runtime,
		NewFunction("", &TypeDef{
			Kind:     TypeDefKindObject,
			AsObject: dagql.NonNull(NewObjectTypeDef("Module", "")),
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to create module definition function for module %q: %w", mod.Name(), err)
	}
	result, err := getModDefFn.Call(ctx, newID, &CallOpts{Cache: true, SkipSelfSchema: true})
	if err != nil {
		return nil, fmt.Errorf("failed to call module %q to get functions: %w", mod.Name(), err)
	}
	inst, ok := result.(dagql.Instance[*Module])
	if !ok {
		return nil, fmt.Errorf("expected Module result, got %T", result)
	}
	newMod := inst.Self.Clone()
	newMod.InstanceID = newID
	return newMod, nil
}

func (mod *Module) Install(ctx context.Context, dag *dagql.Server) error {
	log.Println("!!! INSTALLING MOD", mod.Name())
	defer log.Println("!!! DONE INSTALLING MOD", mod.Name())

	for _, def := range mod.ObjectDefs {
		objDef := def.AsObject.Value
		log.Println("!!! INSTALLING OBJECT", objDef.Name)

		// check whether this is a pre-existing object from a dependency module
		modType, ok, err := mod.Deps.ModTypeFor(ctx, def)
		if err != nil {
			return fmt.Errorf("failed to get mod type for type def: %w", err)
		}

		if ok {
			// XXX(vito): double check this works
			if sourceMod := modType.SourceMod(); sourceMod != nil && sourceMod != mod {
				// modules can reference types from core/other modules as types, but they
				// can't attach any new fields or functions to them
				if len(objDef.Fields) > 0 || len(objDef.Functions) > 0 {
					return fmt.Errorf("cannot attach new fields or functions to object %q from outside module", objDef.Name)
				}
				return nil
			}
		}

		obj := &ModuleObject{
			Module:  mod,
			TypeDef: objDef,
		}

		obj.Install(ctx, dag)
	}

	return nil
}

type ModuleObject struct {
	Module  *Module
	TypeDef *ObjectTypeDef
	Fields  map[string]any
}

func (obj *ModuleObject) Type() *ast.Type {
	return &ast.Type{
		NamedType: obj.TypeDef.Name,
		NonNull:   true,
	}
}

func (obj *ModuleObject) Install(ctx context.Context, dag *dagql.Server) error {
	class := dagql.NewClass(dagql.ClassOpts[*ModuleObject]{
		Typed: obj,
	})
	objDef := obj.TypeDef
	mod := obj.Module
	if objDef.Name == gqlObjectName(mod.Name()) {
		if err := obj.installConstructor(ctx, dag); err != nil {
			return fmt.Errorf("failed to install constructor: %w", err)
		}
	}
	fields := obj.fields()
	funs, err := obj.functions(ctx)
	if err != nil {
		return err
	}
	fields = append(fields, funs...)
	class.Install(fields...)
	dag.InstallObject(class)
	return nil
}

func (obj *ModuleObject) installConstructor(ctx context.Context, dag *dagql.Server) error {
	objDef := obj.TypeDef
	mod := obj.Module

	if !objDef.Constructor.Valid {
		// no constructor defined; install a basic one that initializes an empty
		// object
		dag.Root().ObjectType().Extend(
			dagql.FieldSpec{
				Name:        gqlFieldName(mod.Name()),
				Description: "TODO",
				Type:        obj,
				Pure:        true,
			},
			func(ctx context.Context, self dagql.Object, _ map[string]dagql.Typed) (dagql.Typed, error) {
				return &ModuleObject{
					TypeDef: objDef,
					Fields:  map[string]any{},
				}, nil
			},
		)
		return nil
	}

	// use explicit user-defined constructor if provided
	fnTypeDef := objDef.Constructor.Value
	if fnTypeDef.ReturnType.Kind != TypeDefKindObject {
		return fmt.Errorf("constructor function for object %s must return that object", objDef.OriginalName)
	}
	if fnTypeDef.ReturnType.AsObject.Value.OriginalName != objDef.OriginalName {
		return fmt.Errorf("constructor function for object %s must return that object", objDef.OriginalName)
	}

	fn, err := newModFunction(ctx, mod.Query, mod, mod.InstanceID, objDef, mod.Runtime, fnTypeDef)
	if err != nil {
		return fmt.Errorf("failed to create function: %w", err)
	}

	spec, err := fn.metadata.FieldSpec()
	if err != nil {
		return fmt.Errorf("failed to get field spec: %w", err)
	}

	spec.Name = gqlFieldName(mod.Name())
	spec.Pure = true

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

	return nil
}

func (obj *ModuleObject) fields() (fields []dagql.Field[*ModuleObject]) {
	objDef := obj.TypeDef
	mod := obj.Module
	for _, field := range obj.TypeDef.Fields {
		field := field
		log.Println("!!! INSTALLING FIELD", objDef.Name, field.Name)
		fields = append(fields, dagql.Field[*ModuleObject]{
			Spec: dagql.FieldSpec{
				Name:        field.Name,
				Description: field.Description,
				Type:        field.TypeDef.ToTyped(),
				Pure:        true,
			},
			Func: func(ctx context.Context, obj dagql.Instance[*ModuleObject], _ map[string]dagql.Typed) (dagql.Typed, error) {
				modType, ok, err := mod.ModTypeFor(ctx, field.TypeDef, true)
				if err != nil {
					return nil, fmt.Errorf("failed to get mod type for field %q: %w", field.Name, err)
				}
				if !ok {
					return nil, fmt.Errorf("could not find mod type for field %q", field.Name)
				}
				fieldVal, found := obj.Self.Fields[field.OriginalName]
				if !found {
					return nil, fmt.Errorf("field %q not found on object %q", field.Name, obj.Class.TypeName())
				}
				return modType.ConvertFromSDKResult(ctx, fieldVal)
			},
		})
	}
	return
}

func (obj *ModuleObject) functions(ctx context.Context) (fields []dagql.Field[*ModuleObject], err error) {
	objDef := obj.TypeDef
	mod := obj.Module
	for _, fun := range obj.TypeDef.Functions {
		fun := fun
		log.Println("!!! INSTALLING FUNCTION", objDef.Name, fun.Name)
		modFun, err := newModFunction(
			ctx,
			mod.Query,
			mod,
			mod.InstanceID,
			objDef,
			mod.Runtime,
			fun,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create function %q: %w", fun.Name, err)
		}
		spec, err := fun.FieldSpec()
		if err != nil {
			return nil, fmt.Errorf("failed to get field spec: %w", err)
		}
		fields = append(fields, dagql.Field[*ModuleObject]{
			Spec: spec,
			Func: func(ctx context.Context, obj dagql.Instance[*ModuleObject], args map[string]dagql.Typed) (dagql.Typed, error) {
				opts := &CallOpts{
					ParentVal: obj.Self.Fields,
					Cache:     false, // TODO
					// Pipeline:  _, // TODO
					SkipSelfSchema: false, // TODO?
				}
				for name, val := range args {
					opts.Inputs = append(opts.Inputs, CallInput{
						Name:  name,
						Value: val,
					})
				}
				// NB: ensure deterministic order
				sort.Slice(opts.Inputs, func(i, j int) bool {
					return opts.Inputs[i].Name < opts.Inputs[j].Name
				})
				return modFun.Call(ctx, dagql.CurrentID(ctx), opts)
			},
		})
	}
	return
}

func (obj *ModuleObject) Description() string {
	return formatGqlDescription(obj.TypeDef.Description)
}

func (mod *Module) TypeDefs(ctx context.Context) ([]*TypeDef, error) {
	typeDefs := make([]*TypeDef, 0, len(mod.ObjectDefs))
	for _, obj := range mod.ObjectDefs {
		typeDef := obj.Clone()
		if typeDef.AsObject.Valid {
			typeDef.AsObject.Value.SourceModuleName = mod.Name()
		}
		typeDefs = append(typeDefs, typeDef)
	}
	return typeDefs, nil
}

func (mod *Module) DependencySchemaIntrospectionJSON(ctx context.Context) (string, error) {
	return mod.Deps.SchemaIntrospectionJSON(ctx)
}

func (mod *Module) ModTypeFor(ctx context.Context, typeDef *TypeDef, checkDirectDeps bool) (ModType, bool, error) {
	var modType ModType
	switch typeDef.Kind {
	case TypeDefKindString, TypeDefKindInteger, TypeDefKindBoolean, TypeDefKindVoid:
		modType = &PrimitiveType{typeDef}

	case TypeDefKindList:
		underlyingType, ok, err := mod.ModTypeFor(ctx, typeDef.AsList.Value.ElementTypeDef, checkDirectDeps)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get underlying type: %w", err)
		}
		if !ok {
			return nil, false, nil
		}
		modType = &ListType{
			Elem:       typeDef.AsList.Value.ElementTypeDef,
			Underlying: underlyingType,
		}

	case TypeDefKindObject:
		if checkDirectDeps {
			// check to see if this is from a *direct* dependency
			depType, ok, err := mod.Deps.ModTypeFor(ctx, typeDef)
			if err != nil {
				return nil, false, fmt.Errorf("failed to get type from dependency: %w", err)
			}
			if ok {
				return depType, true, nil
			}
		}

		var found bool
		// otherwise it must be from this module
		for _, obj := range mod.ObjectDefs {
			if obj.AsObject.Value.Name == typeDef.AsObject.Value.Name {
				log.Println("!!! USERMOD FOUND OBJECT", typeDef.AsObject.Value.Name)
				modType = &ModuleObjectType{
					typeDef: obj.AsObject.Value,
					mod:     mod,
				}
				found = true
				break
			}
		}
		if !found {
			log.Println("!!! USERMOD DID NOT FIND OBJECT", typeDef.AsObject.Value.Name)
			return nil, false, nil
		}

	default:
		return nil, false, fmt.Errorf("unexpected type def kind %s", typeDef.Kind)
	}

	if typeDef.Optional {
		modType = &NullableType{
			Elem:       typeDef.WithOptional(false),
			Underlying: modType,
		}
	}

	return modType, true, nil
}

// verify the typedef is has no reserved names
func (mod *Module) validateTypeDef(ctx context.Context, typeDef *TypeDef) error {
	switch typeDef.Kind {
	case TypeDefKindList:
		return mod.validateTypeDef(ctx, typeDef.AsList.Value.ElementTypeDef)
	case TypeDefKindObject:
		obj := typeDef.AsObject.Value

		// check whether this is a pre-existing object from core or another module
		modType, ok, err := mod.Deps.ModTypeFor(ctx, typeDef)
		if err != nil {
			return fmt.Errorf("failed to get mod type for type def: %w", err)
		}
		if ok {
			if sourceMod := modType.SourceMod(); sourceMod != nil && sourceMod != mod {
				// already validated, skip
				return nil
			}
		}

		for _, field := range obj.Fields {
			if gqlFieldName(field.Name) == "id" {
				return fmt.Errorf("cannot define field with reserved name %q on object %q", field.Name, obj.Name)
			}
			fieldType, ok, err := mod.Deps.ModTypeFor(ctx, field.TypeDef)
			if err != nil {
				return fmt.Errorf("failed to get mod type for type def: %w", err)
			}
			if ok {
				if sourceMod := fieldType.SourceMod(); sourceMod != nil && sourceMod.Name() != ModuleName && sourceMod != mod {
					// already validated, skip
					return fmt.Errorf("object %q field %q cannot reference external type from dependency module %q",
						obj.OriginalName,
						field.OriginalName,
						sourceMod.Name(),
					)
				}
			}
			if err := mod.validateTypeDef(ctx, field.TypeDef); err != nil {
				return err
			}
		}

		for _, fn := range obj.Functions {
			if gqlFieldName(fn.Name) == "id" {
				return fmt.Errorf("cannot define function with reserved name %q on object %q", fn.Name, obj.Name)
			}
			// Check if this is a type from another (non-core) module, which is currently not allowed
			retType, ok, err := mod.Deps.ModTypeFor(ctx, fn.ReturnType)
			if err != nil {
				return fmt.Errorf("failed to get mod type for type def: %w", err)
			}
			if ok {
				if sourceMod := retType.SourceMod(); sourceMod != nil && sourceMod.Name() != ModuleName && sourceMod != mod {
					// already validated, skip
					return fmt.Errorf("object %q function %q cannot return external type from dependency module %q",
						obj.OriginalName,
						fn.OriginalName,
						sourceMod.Name(),
					)
				}
			}
			if err := mod.validateTypeDef(ctx, fn.ReturnType); err != nil {
				return err
			}

			for _, arg := range fn.Args {
				if gqlArgName(arg.Name) == "id" {
					return fmt.Errorf("cannot define argument with reserved name %q on function %q", arg.Name, fn.Name)
				}
				argType, ok, err := mod.Deps.ModTypeFor(ctx, arg.TypeDef)
				if err != nil {
					return fmt.Errorf("failed to get mod type for type def: %w", err)
				}
				if ok {
					if sourceMod := argType.SourceMod(); sourceMod != nil && sourceMod.Name() != ModuleName && sourceMod != mod {
						// already validated, skip
						return fmt.Errorf("object %q function %q arg %q cannot reference external type from dependency module %q",
							obj.OriginalName,
							fn.OriginalName,
							arg.OriginalName,
							sourceMod.Name(),
						)
					}
				}
				if err := mod.validateTypeDef(ctx, arg.TypeDef); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// prefix the given typedef (and any recursively referenced typedefs) with this module's name for any objects
func (mod *Module) namespaceTypeDef(ctx context.Context, typeDef *TypeDef) error {
	switch typeDef.Kind {
	case TypeDefKindList:
		if err := mod.namespaceTypeDef(ctx, typeDef.AsList.Value.ElementTypeDef); err != nil {
			return err
		}
	case TypeDefKindObject:
		obj := typeDef.AsObject.Value

		// only namespace objects defined in this module
		_, ok, err := mod.Deps.ModTypeFor(ctx, typeDef)
		if err != nil {
			return fmt.Errorf("failed to get mod type for type def: %w", err)
		}
		if !ok {
			obj.Name = namespaceObject(obj.Name, mod.Name())
		}

		for _, field := range obj.Fields {
			if err := mod.namespaceTypeDef(ctx, field.TypeDef); err != nil {
				return err
			}
		}

		for _, fn := range obj.Functions {
			if err := mod.namespaceTypeDef(ctx, fn.ReturnType); err != nil {
				return err
			}

			for _, arg := range fn.Args {
				if err := mod.namespaceTypeDef(ctx, arg.TypeDef); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

/*
Mod is a module in loaded into the server's DAG of modules; it's the vertex type of the DAG.
It's an interface so we can abstract over user modules and core and treat them the same.
*/
type Mod interface {
	// The name of the module
	Name() string

	// The direct dependencies of this module
	Dependencies() []Mod

	// TODO describe
	Install(context.Context, *dagql.Server) error

	// ModTypeFor returns the ModType for the given typedef based on this module's schema.
	// The returned type will have any namespacing already applied.
	// If checkDirectDeps is true, then its direct dependencies will also be checked.
	ModTypeFor(ctx context.Context, typeDef *TypeDef, checkDirectDeps bool) (ModType, bool, error)

	// All the TypeDefs exposed by this module (does not include dependencies)
	TypeDefs(ctx context.Context) ([]*TypeDef, error)
}

/*
An SDK is an implementation of the functionality needed to generate code for and execute a module.

There is one special SDK, the Go SDK, which is implemented in `goSDK` below. It's used as the "seed" for all
other SDK implementations.

All other SDKs are themselves implemented as Modules, with Functions matching the two defined in this SDK interface.

An SDK Module needs to choose its own SDK for its implementation. This can be "well-known" built-in SDKs like "go",
"python", etc. Or it can be any external module as specified with a module ref.

You can thus think of SDK Modules as a DAG of dependencies, with each SDK using a different SDK to implement its Module,
with the Go SDK as the root of the DAG and the only one without any dependencies.

Built-in SDKs are also a bit special in that they come bundled w/ the engine container image, which allows them
to be used without hard dependencies on the internet. They are loaded w/ the `loadBuiltinSDK` function below, which
loads them as modules from the engine container.
*/
type SDK interface {
	/* Codegen generates code for the module at the given source directory and subpath.

	The Code field of the returned GeneratedCode object should be the generated contents of the module sourceDirSubpath,
	in the case where that's different than the root of the sourceDir.

	The provided Module is not fully initialized; the Runtime field will not be set yet.
	*/
	Codegen(context.Context, *Module, dagql.Instance[*Directory], string) (*GeneratedCode, error)

	/* Runtime returns a container that is used to execute module code at runtime in the Dagger engine.

	The provided Module is not fully initialized; the Runtime field will not be set yet.
	*/
	Runtime(context.Context, *Module, dagql.Instance[*Directory], string) (*Container, error)
}

func (*Module) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Module",
		NonNull:   true,
	}
}

func (mod *Module) PBDefinitions() ([]*pb.Definition, error) {
	var defs []*pb.Definition
	if mod.SourceDirectory.Self != nil {
		dirDefs, err := mod.SourceDirectory.Self.PBDefinitions()
		if err != nil {
			return nil, err
		}
		defs = append(defs, dirDefs...)
	}
	return defs, nil
}

func (mod Module) Clone() *Module {
	cp := mod
	if mod.SourceDirectory.Self != nil {
		cp.SourceDirectory.Self = mod.SourceDirectory.Self.Clone()
	}
	cp.DependencyConfig = cloneSlice(mod.DependencyConfig)
	cp.ObjectDefs = make([]*TypeDef, len(mod.ObjectDefs))
	for i, def := range mod.ObjectDefs {
		cp.ObjectDefs[i] = def.Clone()
	}
	cp.InterfaceDefs = make([]*TypeDef, len(mod.InterfaceDefs))
	for i, def := range mod.InterfaceDefs {
		cp.InterfaceDefs[i] = def.Clone()
	}
	return &cp
}

func (mod *Module) WithObject(ctx context.Context, def *TypeDef) (*Module, error) {
	mod = mod.Clone()
	if !def.AsObject.Valid {
		return nil, fmt.Errorf("expected object type def, got %s: %+v", def.Kind, def)
	}
	if err := mod.validateTypeDef(ctx, def); err != nil {
		return nil, fmt.Errorf("failed to validate type def: %w", err)
	}
	def = def.Clone()
	if err := mod.namespaceTypeDef(ctx, def); err != nil {
		return nil, fmt.Errorf("failed to namespace type def: %w", err)
	}
	mod.ObjectDefs = append(mod.ObjectDefs, def)
	return mod, nil
}

func (mod *Module) WithInterface(ctx context.Context, def *TypeDef) (*Module, error) {
	mod = mod.Clone()
	if !def.AsInterface.Valid {
		return nil, fmt.Errorf("expected interface type def, got %s: %+v", def.Kind, def)
	}
	if err := mod.validateTypeDef(ctx, def); err != nil {
		return nil, fmt.Errorf("failed to validate type def: %w", err)
	}
	def = def.Clone()
	if err := mod.namespaceTypeDef(ctx, def); err != nil {
		return nil, fmt.Errorf("failed to namespace type def: %w", err)
	}
	mod.InterfaceDefs = append(mod.InterfaceDefs, def)
	return mod, nil
}

// Load the module config as parsed from the given File
func LoadModuleConfigFromFile(
	ctx context.Context,
	configFile *File,
) (*modules.Config, error) {
	configBytes, err := configFile.Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg modules.Config
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

// Load the module config from the module from the given diretory at the given path
func LoadModuleConfig(
	ctx context.Context,
	sourceDir *Directory,
	configPath string,
) (string, *modules.Config, error) {
	configPath = modules.NormalizeConfigPath(configPath)
	configFile, err := sourceDir.File(ctx, configPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get config file from path %q: %w", configPath, err)
	}
	cfg, err := LoadModuleConfigFromFile(ctx, configFile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load config: %w", err)
	}
	return configPath, cfg, nil
}

func LoadRef(
	ctx context.Context,
	srv *dagql.Server,
	sourceDir dagql.Instance[*Directory],
	subPath string,
	ref string,
) (dep dagql.Instance[*Module], err error) {
	modRef, err := modules.ResolveStableRef(ref)
	if err != nil {
		return dep, fmt.Errorf("failed to parse dependency url %q: %w", ref, err)
	}
	switch {
	case modRef.Local:
		depPath := filepath.Join(subPath, modRef.Path)
		if strings.HasPrefix(depPath+"/", "../") {
			return dep, fmt.Errorf("local module path %q is not under root", modRef.Path)
		}
		err := srv.Select(ctx, sourceDir, &dep, dagql.Selector{
			Field: "asModule",
			Args: []dagql.NamedInput{
				{Name: "sourceSubpath", Value: dagql.String(depPath)},
			},
		})
		if err != nil {
			return dep, fmt.Errorf("load %q: %w", ref, err)
		}
	case modRef.Git != nil:
		err := srv.Select(ctx, srv.Root(), &dep, dagql.Selector{
			Field: "git",
			Args: []dagql.NamedInput{
				{Name: "url", Value: dagql.String(modRef.Git.CloneURL)},
			},
		}, dagql.Selector{
			Field: "commit",
			Args: []dagql.NamedInput{
				{Name: "id", Value: dagql.String(modRef.Version)},
			},
		}, dagql.Selector{
			Field: "tree",
		}, dagql.Selector{
			Field: "asModule",
			Args: []dagql.NamedInput{
				{Name: "sourceSubpath", Value: dagql.String(modRef.SubPath)},
			},
		})
		if err != nil {
			return dep, fmt.Errorf("load %q: %w", ref, err)
		}
	}
	return
}
