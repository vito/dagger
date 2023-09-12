package schema

import (
	"encoding/json"
	"fmt"

	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/core/resourceid"
	"github.com/dagger/graphql"
	"github.com/dagger/graphql/language/ast"
)

// idResolver is used to generate a scalar resolver for a stringable type.
func idResolver[I resourceid.ID[T], T any]() ScalarResolver {
	return ScalarResolver{
		Serialize: func(value any) any {
			switch v := value.(type) {
			case string:
				return v
			case resourceid.ID[T]:
				return v.String()
			default:
				panic(fmt.Sprintf("want string or resourceid.ID[T], have %T: %+v", v, v))
			}
		},
		ParseValue: func(value any) any {
			switch v := value.(type) {
			case string:
				rid, err := resourceid.Decode(v)
				if err != nil {
					panic(fmt.Errorf("failed to parse resource ID %q: %w", v, err))
				}
				return rid
			default:
				panic(fmt.Sprintf("want string, have %T: %+v", v, v))
			}
		},
		ParseLiteral: func(valueAST ast.Value) any {
			switch v := valueAST.(type) {
			case *ast.StringValue:
				rid, err := resourceid.Decode(v.Value)
				if err != nil {
					panic(fmt.Errorf("failed to parse resource ID %q: %w", v, err))
				}
				return rid
			default:
				panic(fmt.Sprintf("want *ast.StringValue, have %T: %+v", v, v))
			}
		},
	}
}

var jsonResolver = ScalarResolver{
	// serialize object to a JSON string when sending to clients
	Serialize: func(value any) any {
		bs, err := json.Marshal(value)
		if err != nil {
			panic(fmt.Errorf("JSON scalar serialize error: %v", err))
		}
		return string(bs)
	},
	// parse JSON string from clients into the equivalent Go type (string, slice, map, etc.)
	ParseValue: func(value any) any {
		switch v := value.(type) {
		case string:
			if v == "" {
				return nil
			}
			var x any
			if err := json.Unmarshal([]byte(v), &x); err != nil {
				panic(fmt.Errorf("JSON scalar parse value error: %v", err))
			}
			return x
		default:
			panic(fmt.Errorf("JSON scalar parse value unexpected type %T", v))
		}
	},
	ParseLiteral: func(valueAST ast.Value) any {
		switch v := valueAST.(type) {
		case *ast.StringValue:
			var jsonStr string
			if v != nil {
				jsonStr = v.Value
			}
			if jsonStr == "" {
				return nil
			}
			var x any
			if err := json.Unmarshal([]byte(jsonStr), &x); err != nil {
				panic(fmt.Errorf("JSON scalar parse literal error: %v", err))
			}
			return x
		default:
			panic(fmt.Errorf("unexpected literal type for json scalar: %T", valueAST))
		}
	},
}

var voidScalarResolver = ScalarResolver{
	Serialize: func(value any) any {
		if value != nil {
			panic(fmt.Errorf("void scalar serialize unexpected value: %v", value))
		}
		return nil
	},
	ParseValue: func(value any) any {
		if value != nil {
			panic(fmt.Errorf("void scalar parse value unexpected value: %v", value))
		}
		return nil
	},
	ParseLiteral: func(valueAST ast.Value) any {
		if valueAST == nil {
			return nil
		}
		if valueAST.GetValue() != nil {
			panic(fmt.Errorf("void scalar parse literal unexpected value: %v", valueAST.GetValue()))
		}
		return nil
	},
}

func ToVoidResolver[P any, A any](f func(*core.Context, P, A) error) graphql.FieldResolveFn {
	return ToResolver(func(ctx *core.Context, p P, a A) (any, error) {
		return nil, f(ctx, p, a)
	})
}
