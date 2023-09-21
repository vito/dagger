package core

import (
	"encoding/json"
	"fmt"

	"github.com/opencontainers/go-digest"
)

type Function struct {
	IDable

	Name        string         `json:"name"`
	Description string         `json:"description"`
	Args        []*FunctionArg `json:"args"`
	ReturnType  *TypeDef       `json:"returnType"`

	Module *Module `json:"moduleID,omitempty"`
}

func (fn Function) Clone() (*Function, error) {
	cp := fn
	cp.ID = nil
	cp.Args = make([]*FunctionArg, len(fn.Args))
	var err error
	for i, arg := range fn.Args {
		cp.Args[i], err = arg.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone function arg %q: %w", arg.Name, err)
		}
	}
	if fn.ReturnType != nil {
		cp.ReturnType, err = fn.ReturnType.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone return type: %w", err)
		}
	}
	return &cp, nil
}

type FunctionArg struct {
	// IDable // TODO(vito)

	Name         string   `json:"name"`
	Description  string   `json:"description"`
	TypeDef      *TypeDef `json:"typeDef"`
	DefaultValue any      `json:"defaultValue"`
}

func (arg FunctionArg) Clone() (*FunctionArg, error) {
	cp := arg
	// cp.ID = nil // TODO(vito)
	var err error
	cp.TypeDef, err = arg.TypeDef.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone type def: %w", err)
	}

	// TODO: not sure there's any better way to clone any besides a ser/deser cycle
	bs, err := json.Marshal(arg.DefaultValue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default value: %w", err)
	}
	if err := json.Unmarshal(bs, &cp.DefaultValue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default value: %w", err)
	}

	return &cp, nil
}

type TypeDef struct {
	// IDable // TODO(vito)

	Kind     TypeDefKind    `json:"kind"`
	Optional bool           `json:"optional"`
	AsList   *ListTypeDef   `json:"asList"`
	AsObject *ObjectTypeDef `json:"asObject"`
}

func (typeDef TypeDef) Clone() (*TypeDef, error) {
	cp := typeDef
	// cp.ID = nil // TODO(vito)
	if typeDef.AsList != nil {
		var err error
		cp.AsList, err = typeDef.AsList.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone typedef list definition: %w", err)
		}
	}
	if typeDef.AsObject != nil {
		var err error
		cp.AsObject, err = typeDef.AsObject.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone typedef object definition: %w", err)
		}
	}
	return &cp, nil
}

type ObjectTypeDef struct {
	// IDable // TODO(vito)

	Name        string          `json:"name"`
	Description string          `json:"description"`
	Fields      []*FieldTypeDef `json:"fields"`
	Functions   []*Function     `json:"functions"`
}

func (typeDef ObjectTypeDef) Clone() (*ObjectTypeDef, error) {
	cp := typeDef
	// cp.ID = nil // TODO(vito)

	cp.Fields = make([]*FieldTypeDef, len(typeDef.Fields))
	for i, field := range typeDef.Fields {
		var err error
		cp.Fields[i], err = field.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone field %q: %w", field.Name, err)
		}
	}

	cp.Functions = make([]*Function, len(typeDef.Functions))
	for i, fn := range typeDef.Functions {
		var err error
		cp.Functions[i], err = fn.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone function %q: %w", fn.Name, err)
		}
	}

	return &cp, nil
}

func (typeDef ObjectTypeDef) FieldByName(name string) (*FieldTypeDef, bool) {
	for _, field := range typeDef.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return nil, false
}

func (typeDef ObjectTypeDef) FunctionByName(name string) (*Function, bool) {
	for _, fn := range typeDef.Functions {
		if fn.Name == name {
			return fn, true
		}
	}
	return nil, false
}

type FieldTypeDef struct {
	// IDable // TODO(vito)

	Name        string   `json:"name"`
	Description string   `json:"description"`
	TypeDef     *TypeDef `json:"typeDef"`
}

func (typeDef FieldTypeDef) Clone() (*FieldTypeDef, error) {
	cp := typeDef
	// cp.ID = nil // TODO(vito)
	if typeDef.TypeDef != nil {
		var err error
		cp.TypeDef, err = typeDef.TypeDef.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone field typedef: %w", err)
		}
	}
	return &cp, nil
}

type ListTypeDef struct {
	// IDable                  // TODO(vito)

	ElementTypeDef *TypeDef `json:"elementTypeDef"`
}

func (typeDef ListTypeDef) Clone() (*ListTypeDef, error) {
	cp := typeDef
	// cp.ID = nil // TODO(vito)
	if typeDef.ElementTypeDef != nil {
		var err error
		cp.ElementTypeDef, err = typeDef.ElementTypeDef.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone list element typedef: %w", err)
		}
	}
	return &cp, nil
}

type TypeDefKind string

func (k TypeDefKind) String() string {
	return string(k)
}

const (
	TypeDefKindString  TypeDefKind = "StringKind"
	TypeDefKindInteger TypeDefKind = "IntegerKind"
	TypeDefKindBoolean TypeDefKind = "BooleanKind"
	TypeDefKindList    TypeDefKind = "ListKind"
	TypeDefKindObject  TypeDefKind = "ObjectKind"
	TypeDefKindVoid    TypeDefKind = "VoidKind"
)

type FunctionCall struct {
	Name       string       `json:"name"`
	ParentName string       `json:"parentName"`
	Parent     any          `json:"parent"`
	InputArgs  []*CallInput `json:"inputArgs"`
}

func (fnCall *FunctionCall) Digest() (digest.Digest, error) {
	return stableDigest(fnCall)
}

type CallInput struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}
