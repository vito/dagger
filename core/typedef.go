package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vito/dagql"
	"github.com/vito/dagql/idproto"
)

type Function struct {
	// Name is the standardized name of the function (lowerCamelCase), as used for the resolver in the graphql schema
	Name        string         `field:"true"`
	Description string         `field:"true"`
	Args        []*FunctionArg `field:"true"`
	ReturnType  *TypeDef       `field:"true"`

	// Below are not in public API

	// OriginalName of the parent object
	ParentOriginalName string

	// The original name of the function as provided by the SDK that defined it, used
	// when invoking the SDK so it doesn't need to think as hard about case conversions
	OriginalName string
}

func NewFunction(name string, returnType *TypeDef) *Function {
	return &Function{
		Name:         strcase.ToLowerCamel(name),
		ReturnType:   returnType,
		OriginalName: name,
	}
}

func (*Function) Type() *ast.Type {
	return &ast.Type{
		NamedType: "Function",
		NonNull:   true,
	}
}

func (fn Function) Clone() *Function {
	cp := fn
	cp.Args = make([]*FunctionArg, len(fn.Args))
	for i, arg := range fn.Args {
		cp.Args[i] = arg.Clone()
	}
	if fn.ReturnType != nil {
		cp.ReturnType = fn.ReturnType.Clone()
	}
	return &cp
}

func (fn *Function) FieldSpec() (dagql.FieldSpec, error) {
	spec := dagql.FieldSpec{
		Name:        fn.Name,
		Description: formatGqlDescription(fn.Description),
		Type:        fn.ReturnType.ToTyped(),
		Pure:        false, // TODO
	}
	for _, arg := range fn.Args {
		input := arg.TypeDef.ToInput()
		var defaultVal dagql.Input
		if arg.DefaultValue != nil {
			var val any
			dec := json.NewDecoder(bytes.NewReader(arg.DefaultValue.Bytes()))
			dec.UseNumber()
			if err := dec.Decode(&val); err != nil {
				return spec, fmt.Errorf("failed to decode default value for arg %q: %w", arg.Name, err)
			}
			var err error
			defaultVal, err = input.Decoder().DecodeInput(val)
			if err != nil {
				return spec, fmt.Errorf("failed to decode default value for arg %q: %w", arg.Name, err)
			}
		}
		spec.Args = append(spec.Args, dagql.InputSpec{
			Name:        arg.Name,
			Description: formatGqlDescription(arg.Description),
			Type:        input,
			Default:     defaultVal,
		})
	}
	return spec, nil
}

func (fn *Function) WithDescription(desc string) *Function {
	fn = fn.Clone()
	fn.Description = strings.TrimSpace(desc)
	return fn
}

func (fn *Function) WithArg(name string, typeDef *TypeDef, desc string, defaultValue JSON) *Function {
	fn = fn.Clone()
	fn.Args = append(fn.Args, &FunctionArg{
		Name:         strcase.ToLowerCamel(name),
		Description:  desc,
		TypeDef:      typeDef,
		DefaultValue: defaultValue,
		OriginalName: name,
	})
	return fn
}

func (fn *Function) IsSubtypeOf(otherFn *Function) bool {
	if fn == nil || otherFn == nil {
		return false
	}

	// check return type
	if !fn.ReturnType.IsSubtypeOf(otherFn.ReturnType) {
		return false
	}

	// check args
	for i, otherFnArg := range otherFn.Args {
		/* TODO: with more effort could probably relax and allow:
		* arg names to not match (only types really matter in theory)
		* mismatches in optional (provided defaults exist, etc.)
		* fewer args in interface fn than object fn (as long as the ones that exist match)
		 */

		if i >= len(fn.Args) {
			return false
		}
		fnArg := fn.Args[i]

		if fnArg.Name != otherFnArg.Name {
			return false
		}

		if fnArg.TypeDef.Optional != otherFnArg.TypeDef.Optional {
			return false
		}

		// We want to be contravariant on arg matching types. So if fnArg asks for a Cat, then
		// we can't invoke it with any Animal since it requested a cat specifically.
		// However, if the fnArg asks for an Animal, we can provide a Cat because that's a subtype of Animal.
		// Thus, we check that the otherFnArg is a subtype of the fnArg (inverse of the covariant matching done
		// on function *return* types above).
		if !otherFnArg.TypeDef.IsSubtypeOf(fnArg.TypeDef) {
			return false
		}
	}

	return true
}

func (fn *Function) LookupArg(name string) (*FunctionArg, bool) {
	for _, arg := range fn.Args {
		if arg.Name == name {
			return arg, true
		}
	}
	return nil, false
}

type FunctionArg struct {
	// Name is the standardized name of the argument (lowerCamelCase), as used for the resolver in the graphql schema
	Name         string   `field:"true"`
	Description  string   `field:"true"`
	TypeDef      *TypeDef `field:"true"`
	DefaultValue JSON     `field:"true"`

	// Below are not in public API

	// The original name of the argument as provided by the SDK that defined it.
	OriginalName string
}

// Type returns the GraphQL FunctionArg! type.
func (*FunctionArg) Type() *ast.Type {
	return &ast.Type{
		NamedType: "FunctionArg",
		NonNull:   true,
	}
}

type DynamicID struct {
	objDef *ObjectTypeDef
	id     *idproto.ID
}

var _ dagql.IDable = DynamicID{}

// ID returns the ID of the value.
func (i DynamicID) ID() *idproto.ID {
	return i.id
}

var _ dagql.ScalarType = DynamicID{}

func (d DynamicID) TypeName() string {
	return fmt.Sprintf("%sID", d.objDef.Name)
}

var _ dagql.InputDecoder = DynamicID{}

func (d DynamicID) DecodeInput(val any) (dagql.Input, error) {
	switch x := val.(type) {
	case string:
		var idp idproto.ID
		if err := idp.Decode(x); err != nil {
			return nil, fmt.Errorf("decode %q ID: %w", d.objDef.Name, err)
		}
		d.id = &idp
		return d, nil
	default:
		return nil, fmt.Errorf("expected string, got %T", val)
	}
}

var _ dagql.Input = DynamicID{}

func (d DynamicID) ToLiteral() *idproto.Literal {
	return &idproto.Literal{
		Value: &idproto.Literal_Id{
			Id: d.id,
		},
	}
}

func (d DynamicID) Type() *ast.Type {
	return &ast.Type{
		NamedType: d.TypeName(),
		NonNull:   true,
	}
}

func (d DynamicID) Decoder() dagql.InputDecoder {
	return DynamicID{
		objDef: d.objDef,
	}
}

func (i DynamicID) MarshalJSON() ([]byte, error) {
	enc, err := i.id.Encode()
	if err != nil {
		return nil, err
	}
	return json.Marshal(enc)
}

func (arg FunctionArg) Clone() *FunctionArg {
	cp := arg
	cp.TypeDef = arg.TypeDef.Clone()
	// NB(vito): don't bother copying DefaultValue, it's already 'any' so it's
	// hard to imagine anything actually mutating it at runtime vs. replacing it
	// wholesale.
	return &cp
}

type TypeDef struct {
	Kind        TypeDefKind                       `field:"true"`
	Optional    bool                              `field:"true"`
	AsList      dagql.Nullable[*ListTypeDef]      `field:"true"`
	AsObject    dagql.Nullable[*ObjectTypeDef]    `field:"true"`
	AsInterface dagql.Nullable[*InterfaceTypeDef] `field:"true"`
}

func (*TypeDef) Type() *ast.Type {
	return &ast.Type{
		NamedType: "TypeDef",
		NonNull:   true,
	}
}

func (t TypeDef) ToTyped() dagql.Typed {
	var typed dagql.Typed
	switch t.Kind {
	case TypeDefKindString:
		typed = dagql.String("")
	case TypeDefKindInteger:
		typed = dagql.Int(0)
	case TypeDefKindBoolean:
		typed = dagql.Boolean(false)
	case TypeDefKindList:
		typed = dagql.DynamicArrayOutput{Elem: t.AsList.Value.ElementTypeDef.ToTyped()}
	case TypeDefKindObject:
		typed = &ModuleObject{TypeDef: t.AsObject.Value}
	case TypeDefKindVoid:
		typed = Void{}
	default:
		panic(fmt.Sprintf("unknown type kind: %s", t.Kind))
	}
	if t.Optional {
		typed = dagql.DynamicNullable{Elem: typed}
	}
	return typed
}

func (t TypeDef) ToInput() dagql.Input {
	var typed dagql.Input
	switch t.Kind {
	case TypeDefKindString:
		typed = dagql.String("")
	case TypeDefKindInteger:
		typed = dagql.Int(0)
	case TypeDefKindBoolean:
		typed = dagql.Boolean(false)
	case TypeDefKindList:
		typed = dagql.DynamicArrayInput{Elem: t.AsList.Value.ElementTypeDef.ToInput()}
	case TypeDefKindObject:
		typed = DynamicID{objDef: t.AsObject.Value}
	case TypeDefKindVoid:
		typed = Void{}
	default:
		panic(fmt.Sprintf("unknown type kind: %s", t.Kind))
	}
	if t.Optional {
		typed = dagql.DynamicOptional{Elem: typed}
	}
	return typed
}

func (t TypeDef) ToType() *ast.Type {
	return t.ToTyped().Type()
}

func (typeDef *TypeDef) Underlying() *TypeDef {
	switch typeDef.Kind {
	case TypeDefKindList:
		return typeDef.AsList.Value.ElementTypeDef.Underlying()
	default:
		return typeDef
	}
}

func (typeDef TypeDef) Clone() *TypeDef {
	cp := typeDef
	if typeDef.AsList.Valid {
		cp.AsList.Value = typeDef.AsList.Value.Clone()
	}
	if typeDef.AsObject.Valid {
		cp.AsObject.Value = typeDef.AsObject.Value.Clone()
	}
	if typeDef.AsInterface.Valid {
		cp.AsInterface.Value = typeDef.AsInterface.Value.Clone()
	}
	return &cp
}

func (typeDef *TypeDef) WithKind(kind TypeDefKind) *TypeDef {
	typeDef = typeDef.Clone()
	typeDef.Kind = kind
	return typeDef
}

func (typeDef *TypeDef) WithListOf(elem *TypeDef) *TypeDef {
	typeDef = typeDef.WithKind(TypeDefKindList)
	typeDef.AsList = dagql.NonNull(&ListTypeDef{
		ElementTypeDef: elem,
	})
	return typeDef
}

func (typeDef *TypeDef) WithObject(name, desc string) *TypeDef {
	typeDef = typeDef.WithKind(TypeDefKindObject)
	typeDef.AsObject = dagql.NonNull(NewObjectTypeDef(name, desc))
	return typeDef
}

func (typeDef *TypeDef) WithInterface(name, desc string) *TypeDef {
	typeDef = typeDef.WithKind(TypeDefKindInterface)
	typeDef.AsInterface = dagql.NonNull(NewInterfaceTypeDef(name, desc))
	return typeDef
}

func (typeDef *TypeDef) WithOptional(optional bool) *TypeDef {
	typeDef = typeDef.Clone()
	typeDef.Optional = optional
	return typeDef
}

func (typeDef *TypeDef) WithObjectField(name string, fieldType *TypeDef, desc string) (*TypeDef, error) {
	if !typeDef.AsObject.Valid {
		return nil, fmt.Errorf("cannot add function to non-object type: %s", typeDef.Kind)
	}
	typeDef = typeDef.Clone()
	typeDef.AsObject.Value.Fields = append(typeDef.AsObject.Value.Fields, &FieldTypeDef{
		Name:         strcase.ToLowerCamel(name),
		OriginalName: name,
		Description:  desc,
		TypeDef:      fieldType,
	})
	return typeDef, nil
}

func (typeDef *TypeDef) WithFunction(fn *Function) (*TypeDef, error) {
	typeDef = typeDef.Clone()
	fn = fn.Clone()
	switch typeDef.Kind {
	case TypeDefKindObject:
		fn.ParentOriginalName = typeDef.AsObject.Value.OriginalName
		typeDef.AsObject.Value.Functions = append(typeDef.AsObject.Value.Functions, fn)
		return typeDef, nil
	case TypeDefKindInterface:
		fn.ParentOriginalName = typeDef.AsInterface.Value.OriginalName
		typeDef.AsInterface.Value.Functions = append(typeDef.AsInterface.Value.Functions, fn)
		return typeDef, nil
	default:
		return nil, fmt.Errorf("cannot add function to type: %s", typeDef.Kind)
	}
}

func (typeDef *TypeDef) WithObjectConstructor(fn *Function) (*TypeDef, error) {
	if !typeDef.AsObject.Valid {
		return nil, fmt.Errorf("cannot add constructor function to non-object type: %s", typeDef.Kind)
	}

	typeDef = typeDef.Clone()
	fn = fn.Clone()
	fn.ParentOriginalName = typeDef.AsObject.Value.OriginalName
	typeDef.AsObject.Value.Constructor = dagql.NonNull(fn)
	return typeDef, nil
}

func (typeDef *TypeDef) IsSubtypeOf(otherDef *TypeDef) bool {
	if typeDef == nil || otherDef == nil {
		return false
	}

	if typeDef.Optional != otherDef.Optional {
		return false
	}

	switch typeDef.Kind {
	case TypeDefKindString, TypeDefKindInteger, TypeDefKindBoolean, TypeDefKindVoid:
		return typeDef.Kind == otherDef.Kind
	case TypeDefKindList:
		if otherDef.Kind != TypeDefKindList {
			return false
		}
		return typeDef.AsList.Value.ElementTypeDef.IsSubtypeOf(otherDef.AsList.Value.ElementTypeDef)
	case TypeDefKindObject:
		switch otherDef.Kind {
		case TypeDefKindObject:
			// For now, assume that if the objects have the same name, they are the same object. This should be a safe assumption
			// within the context of a single, already-namedspace schema, but not safe if objects are compared across schemas
			return typeDef.AsObject.Value.Name == otherDef.AsObject.Value.Name
		case TypeDefKindInterface:
			return typeDef.AsObject.Value.IsSubtypeOf(otherDef.AsInterface.Value)
		default:
			return false
		}
	case TypeDefKindInterface:
		if otherDef.Kind != TypeDefKindInterface {
			return false
		}
		return typeDef.AsInterface.Value.IsSubtypeOf(otherDef.AsInterface.Value)
	default:
		return false
	}
}

type ObjectTypeDef struct {
	// Name is the standardized name of the object (CamelCase), as used for the object in the graphql schema
	Name        string                    `field:"true"`
	Description string                    `field:"true"`
	Fields      []*FieldTypeDef           `field:"true"`
	Functions   []*Function               `field:"true"`
	Constructor dagql.Nullable[*Function] `field:"true"`

	// SourceModuleName is currently only set when returning the TypeDef from the Objects field on Module
	SourceModuleName string `field:"true"`

	// Below are not in public API

	// The original name of the object as provided by the SDK that defined it, used
	// when invoking the SDK so it doesn't need to think as hard about case conversions
	OriginalName string
}

func (*ObjectTypeDef) Type() *ast.Type {
	return &ast.Type{
		NamedType: "ObjectTypeDef",
		NonNull:   true,
	}
}

func NewObjectTypeDef(name, description string) *ObjectTypeDef {
	if name == "" {
		panic("WHY WOULD I HAVE NO NAME")
	}
	return &ObjectTypeDef{
		Name:         strcase.ToCamel(name),
		OriginalName: name,
		Description:  description,
	}
}

func (obj ObjectTypeDef) Clone() *ObjectTypeDef {
	cp := obj

	cp.Fields = make([]*FieldTypeDef, len(obj.Fields))
	for i, field := range obj.Fields {
		cp.Fields[i] = field.Clone()
	}

	cp.Functions = make([]*Function, len(obj.Functions))
	for i, fn := range obj.Functions {
		cp.Functions[i] = fn.Clone()
	}

	if cp.Constructor.Valid {
		cp.Constructor.Value = obj.Constructor.Value.Clone()
	}

	return &cp
}

func (obj ObjectTypeDef) FieldByName(name string) (*FieldTypeDef, bool) {
	for _, field := range obj.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return nil, false
}

func (obj ObjectTypeDef) FunctionByName(name string) (*Function, bool) {
	for _, fn := range obj.Functions {
		if fn.Name == name {
			return fn, true
		}
	}
	return nil, false
}

func (obj *ObjectTypeDef) IsSubtypeOf(iface *InterfaceTypeDef) bool {
	if obj == nil || iface == nil {
		return false
	}

	objFnByName := make(map[string]*Function)
	for _, fn := range obj.Functions {
		objFnByName[fn.Name] = fn
	}
	objFieldByName := make(map[string]*FieldTypeDef)
	for _, field := range obj.Fields {
		objFieldByName[field.Name] = field
	}

	for _, ifaceFn := range iface.Functions {
		objFn, objFnExists := objFnByName[ifaceFn.Name]
		objField, objFieldExists := objFieldByName[ifaceFn.Name]

		if !objFnExists && !objFieldExists {
			return false
		}

		if objFieldExists {
			// check return type of field
			return objField.TypeDef.IsSubtypeOf(ifaceFn.ReturnType)
		}

		// otherwise there can only be a match on the objFn
		if ok := objFn.IsSubtypeOf(ifaceFn); !ok {
			return false
		}
	}

	return true
}

type FieldTypeDef struct {
	Name        string   `field:"true"`
	Description string   `field:"true"`
	TypeDef     *TypeDef `field:"true"`

	// Below are not in public API

	// The original name of the object as provided by the SDK that defined it, used
	// when invoking the SDK so it doesn't need to think as hard about case conversions
	OriginalName string
}

func (*FieldTypeDef) Type() *ast.Type {
	return &ast.Type{
		NamedType: "FieldTypeDef",
		NonNull:   true,
	}
}

func (typeDef FieldTypeDef) Clone() *FieldTypeDef {
	cp := typeDef
	if typeDef.TypeDef != nil {
		cp.TypeDef = typeDef.TypeDef.Clone()
	}
	return &cp
}

type InterfaceTypeDef struct {
	// Name is the standardized name of the interface (CamelCase), as used for the interface in the graphql schema
	Name        string      `field:"true"`
	Description string      `field:"true"`
	Functions   []*Function `field:"true"`
	// SourceModuleName is currently only set when returning the TypeDef from the Objects field on Module
	SourceModuleName string `field:"true"`

	// Below are not in public API

	// The original name of the interface as provided by the SDK that defined it, used
	// when invoking the SDK so it doesn't need to think as hard about case conversions
	OriginalName string
}

func NewInterfaceTypeDef(name, description string) *InterfaceTypeDef {
	return &InterfaceTypeDef{
		Name:         strcase.ToCamel(name),
		OriginalName: name,
		Description:  description,
	}
}

func (*InterfaceTypeDef) Type() *ast.Type {
	return &ast.Type{
		NamedType: "InterfaceTypeDef",
		NonNull:   true,
	}
}

func (iface InterfaceTypeDef) Clone() *InterfaceTypeDef {
	cp := iface

	cp.Functions = make([]*Function, len(iface.Functions))
	for i, fn := range iface.Functions {
		cp.Functions[i] = fn.Clone()
	}

	return &cp
}

func (iface *InterfaceTypeDef) IsSubtypeOf(otherIface *InterfaceTypeDef) bool {
	if iface == nil || otherIface == nil {
		return false
	}

	ifaceFnByName := make(map[string]*Function)
	for _, fn := range iface.Functions {
		ifaceFnByName[fn.Name] = fn
	}

	for _, otherIfaceFn := range otherIface.Functions {
		ifaceFn, ok := ifaceFnByName[otherIfaceFn.Name]
		if !ok {
			return false
		}

		if ok := ifaceFn.IsSubtypeOf(otherIfaceFn); !ok {
			return false
		}
	}

	return true
}

type ListTypeDef struct {
	ElementTypeDef *TypeDef `field:"true"`
}

func (*ListTypeDef) Type() *ast.Type {
	return &ast.Type{
		NamedType: "ListTypeDef",
		NonNull:   true,
	}
}

func (typeDef ListTypeDef) Clone() *ListTypeDef {
	cp := typeDef
	if typeDef.ElementTypeDef != nil {
		cp.ElementTypeDef = typeDef.ElementTypeDef.Clone()
	}
	return &cp
}

type TypeDefKind string

func (k TypeDefKind) String() string {
	return string(k)
}

var TypeDefKinds = dagql.NewEnum[TypeDefKind]()

var (
	TypeDefKindString    = TypeDefKinds.Register("StringKind")
	TypeDefKindInteger   = TypeDefKinds.Register("IntegerKind")
	TypeDefKindBoolean   = TypeDefKinds.Register("BooleanKind")
	TypeDefKindList      = TypeDefKinds.Register("ListKind")
	TypeDefKindObject    = TypeDefKinds.Register("ObjectKind")
	TypeDefKindInterface = TypeDefKinds.Register("InterfaceKind")
	TypeDefKindVoid      = TypeDefKinds.Register("VoidKind")
)

func (proto TypeDefKind) Type() *ast.Type {
	return &ast.Type{
		NamedType: "TypeDefKind",
		NonNull:   true,
	}
}

func (proto TypeDefKind) Decoder() dagql.InputDecoder {
	return TypeDefKinds
}

func (proto TypeDefKind) ToLiteral() *idproto.Literal {
	return TypeDefKinds.Literal(proto)
}

type FunctionCall struct {
	Query *Query

	Name       string `field:"true"`
	ParentName string `field:"true"`
	Parent     JSON
	InputArgs  []*FunctionCallArgValue `field:"true"`
}

func (*FunctionCall) Type() *ast.Type {
	return &ast.Type{
		NamedType: "FunctionCall",
		NonNull:   true,
	}
}

func (fnCall *FunctionCall) ReturnValue(ctx context.Context, val JSON) error {
	// The return is implemented by exporting the result back to the caller's
	// filesystem. This ensures that the result is cached as part of the module
	// function's Exec while also keeping SDKs as agnostic as possible to the
	// format + location of that result.
	return fnCall.Query.Buildkit.IOReaderExport(
		ctx,
		bytes.NewReader(val),
		filepath.Join(modMetaDirPath, modMetaOutputPath),
		0600,
	)
}

type FunctionCallArgValue struct {
	Name  string `field:"true"`
	Value JSON   `field:"true"`
}

func (*FunctionCallArgValue) Type() *ast.Type {
	return &ast.Type{
		NamedType: "FunctionCallArgValue",
		NonNull:   true,
	}
}
