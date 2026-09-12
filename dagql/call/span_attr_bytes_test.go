package call

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/dagger/dagger/dagql/call/callpbv1"
)

func bytesLiteral(size int) *callpbv1.Literal {
	return &callpbv1.Literal{Value: &callpbv1.Literal_Bytes{Bytes: make([]byte, size)}}
}

func TestHasOversizedBytesLiterals(t *testing.T) {
	big := MaxSpanAttrBytesLiteral + 1

	for name, test := range map[string]struct {
		call *callpbv1.Call
		want bool
	}{
		"nil": {nil, false},
		"no args": {
			&callpbv1.Call{Field: "host"}, false,
		},
		"threshold-sized bytes pass": {
			&callpbv1.Call{Args: []*callpbv1.Argument{
				{Name: "contents", Value: bytesLiteral(MaxSpanAttrBytesLiteral)},
			}}, false,
		},
		"oversized bytes arg": {
			&callpbv1.Call{Args: []*callpbv1.Argument{
				{Name: "contents", Value: bytesLiteral(big)},
			}}, true,
		},
		"oversized bytes in implicit input": {
			&callpbv1.Call{ImplicitInputs: []*callpbv1.Argument{
				{Name: "seed", Value: bytesLiteral(big)},
			}}, true,
		},
		"oversized bytes nested in list": {
			&callpbv1.Call{Args: []*callpbv1.Argument{{
				Name: "files",
				Value: &callpbv1.Literal{Value: &callpbv1.Literal_List{List: &callpbv1.List{Values: []*callpbv1.Literal{
					{Value: &callpbv1.Literal_String_{String_: "keep"}},
					bytesLiteral(big),
				}}}},
			}}}, true,
		},
		"oversized bytes nested in object": {
			&callpbv1.Call{Args: []*callpbv1.Argument{{
				Name: "config",
				Value: &callpbv1.Literal{Value: &callpbv1.Literal_Object{Object: &callpbv1.Object{Values: []*callpbv1.Argument{
					{Name: "data", Value: bytesLiteral(big)},
				}}}},
			}}}, true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := HasOversizedBytesLiterals(test.call); got != test.want {
				t.Fatalf("HasOversizedBytesLiterals = %v, want %v", got, test.want)
			}
		})
	}
}

// A large non-bytes literal is not the span attribute's problem: only raw
// byte payloads are barred from riding it.
func TestHasOversizedBytesLiteralsIgnoresLargeStrings(t *testing.T) {
	id := New().Append(
		&ast.Type{NamedType: "Directory", NonNull: true},
		"directory",
		WithArgs(NewArgument("path", NewLiteralString(string(make([]byte, 2*MaxSpanAttrBytesLiteral))), false)),
	)
	if HasOversizedBytesLiterals(id.Call()) {
		t.Fatal("string literals must not trip the bytes guard")
	}
}
