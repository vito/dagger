package call

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"google.golang.org/protobuf/proto"

	"github.com/dagger/dagger/dagql/call/callpbv1"
)

func bigTelemetryBytes(canary string) []byte {
	value := make([]byte, MaxTelemetryBytesLiteral+1)
	copy(value, canary)
	return value
}

// The telemetry copy of a call frame must not carry large raw-byte arguments:
// those bytes flow to OTLP exporters and per-client telemetry DBs, a consent
// and size boundary the recipe itself never crosses.
func TestScrubbedCallForTelemetryReplacesLargeBytes(t *testing.T) {
	const canary = "TELEMETRY-CANARY-must-not-leak"
	contents := bigTelemetryBytes(canary)

	id := New().Append(
		&ast.Type{NamedType: "File", NonNull: true},
		"blob",
		WithArgs(NewArgument("contents", NewLiteralBytes(contents), false)),
	)
	callPB := id.Call()

	scrubbed := ScrubbedCallForTelemetry(callPB)
	if scrubbed == callPB {
		t.Fatal("a frame with a large bytes literal must be copied, not returned as-is")
	}

	// Everything but the argument bytes is preserved — most importantly the
	// digest, which is how consumers key the frame.
	if scrubbed.GetDigest() != callPB.GetDigest() {
		t.Fatalf("scrub changed the carried digest: got %s, want %s", scrubbed.GetDigest(), callPB.GetDigest())
	}
	if scrubbed.GetField() != "blob" {
		t.Fatalf("scrub changed the field: %q", scrubbed.GetField())
	}

	arg := scrubbed.GetArgs()[0]
	if arg.GetName() != "contents" {
		t.Fatalf("scrub changed the arg name: %q", arg.GetName())
	}
	placeholder, ok := arg.GetValue().GetValue().(*callpbv1.Literal_String_)
	if !ok {
		t.Fatalf("scrubbed literal has type %T, want string placeholder", arg.GetValue().GetValue())
	}
	if want := DisplayBytes(contents); placeholder.String_ != want {
		t.Fatalf("placeholder = %q, want %q", placeholder.String_, want)
	}

	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(scrubbed)
	if err != nil {
		t.Fatalf("marshal scrubbed frame: %v", err)
	}
	if bytes.Contains(payload, []byte(canary)) {
		t.Fatal("scrubbed telemetry payload still contains the raw bytes")
	}
	if len(payload) > MaxTelemetryBytesLiteral {
		t.Fatalf("scrubbed telemetry payload is %d bytes, expected it to shrink below %d", len(payload), MaxTelemetryBytesLiteral)
	}

	// The input is shared with the live ID and must never be mutated: the
	// recipe itself keeps the raw bytes.
	if got := callPB.GetArgs()[0].GetValue().GetBytes(); !bytes.Equal(got, contents) {
		t.Fatal("scrub mutated the original call frame")
	}
	if id.Arg("contents").Value().Display() != DisplayBytes(contents) {
		t.Fatal("scrub disturbed the live ID")
	}
}

func TestScrubbedCallForTelemetryKeepsSmallBytes(t *testing.T) {
	contents := make([]byte, MaxTelemetryBytesLiteral)
	id := New().Append(
		&ast.Type{NamedType: "File", NonNull: true},
		"blob",
		WithArgs(NewArgument("contents", NewLiteralBytes(contents), false)),
	)
	callPB := id.Call()
	if scrubbed := ScrubbedCallForTelemetry(callPB); scrubbed != callPB {
		t.Fatal("a frame with only threshold-sized bytes must be returned unchanged")
	}
}

func TestScrubbedCallForTelemetryHandlesNestedLiterals(t *testing.T) {
	const canary = "NESTED-TELEMETRY-CANARY"
	big := bigTelemetryBytes(canary)

	keep := &callpbv1.Literal{Value: &callpbv1.Literal_String_{String_: "keep"}}
	callPB := &callpbv1.Call{
		Field:  "withFiles",
		Digest: "xxh3:nested-test",
		Args: []*callpbv1.Argument{{
			Name: "files",
			Value: &callpbv1.Literal{Value: &callpbv1.Literal_List{List: &callpbv1.List{Values: []*callpbv1.Literal{
				{Value: &callpbv1.Literal_Bytes{Bytes: big}},
				keep,
			}}}},
		}},
		ImplicitInputs: []*callpbv1.Argument{{
			Name: "seed",
			Value: &callpbv1.Literal{Value: &callpbv1.Literal_Object{Object: &callpbv1.Object{Values: []*callpbv1.Argument{
				{Name: "data", Value: &callpbv1.Literal{Value: &callpbv1.Literal_Bytes{Bytes: big}}},
			}}}},
		}},
	}

	scrubbed := ScrubbedCallForTelemetry(callPB)
	if scrubbed == callPB {
		t.Fatal("nested large bytes literals must trigger a scrub")
	}

	listValues := scrubbed.GetArgs()[0].GetValue().GetList().GetValues()
	if got := listValues[0].GetString_(); got != DisplayBytes(big) {
		t.Fatalf("list element not scrubbed: %q", got)
	}
	if listValues[1] != keep {
		t.Fatal("unchanged list element should be shared, not copied")
	}

	objValues := scrubbed.GetImplicitInputs()[0].GetValue().GetObject().GetValues()
	if got := objValues[0].GetValue().GetString_(); got != DisplayBytes(big) {
		t.Fatalf("object field not scrubbed: %q", got)
	}

	rendered, err := proto.MarshalOptions{Deterministic: true}.Marshal(scrubbed)
	if err != nil {
		t.Fatalf("marshal scrubbed frame: %v", err)
	}
	if bytes.Contains(rendered, []byte(canary)) {
		t.Fatal("nested raw bytes leaked through the scrub")
	}

	// The original message keeps its bytes in both nests.
	if !bytes.Equal(callPB.GetArgs()[0].GetValue().GetList().GetValues()[0].GetBytes(), big) {
		t.Fatal("scrub mutated the original list literal")
	}
	if !bytes.Equal(callPB.GetImplicitInputs()[0].GetValue().GetObject().GetValues()[0].GetValue().GetBytes(), big) {
		t.Fatal("scrub mutated the original object literal")
	}
}

func TestScrubbedCallForTelemetryLeavesByteFreeFramesAlone(t *testing.T) {
	id := New().Append(
		&ast.Type{NamedType: "Directory", NonNull: true},
		"directory",
		WithArgs(NewArgument("path", NewLiteralString(strings.Repeat("p", 2*MaxTelemetryBytesLiteral)), false)),
	)
	callPB := id.Call()
	if scrubbed := ScrubbedCallForTelemetry(callPB); scrubbed != callPB {
		t.Fatal("a frame without bytes literals must be returned as-is")
	}
	if ScrubbedCallForTelemetry(nil) != nil {
		t.Fatal("nil in, nil out")
	}
}
