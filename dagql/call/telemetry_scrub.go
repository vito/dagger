package call

import (
	"slices"

	"github.com/dagger/dagger/dagql/call/callpbv1"
)

// MaxTelemetryBytesLiteral is the largest raw Bytes literal a telemetry call
// payload may carry verbatim. Larger values — e.g. a workspace snapshot's git
// bundle passed to Query.blob — are replaced with their digest+size
// placeholder before emission: telemetry fans out to OTLP exporters and
// per-client telemetry DBs, a very different consent and size boundary than
// the engine-local recipe the bytes actually belong to.
const MaxTelemetryBytesLiteral = 1 << 10 // 1 KiB

// ScrubbedCallForTelemetry returns a copy of callPB safe to emit through
// telemetry: every Bytes literal larger than MaxTelemetryBytesLiteral —
// including ones nested in list and object literals — is replaced with a
// String literal holding the digest+size rendering LiteralBytes.Display
// already uses.
//
// This is strictly a telemetry-side transform. The input is never mutated
// (the callpbv1 messages are shared with the live ID — see ID.ToProto), and
// real ID serialization (LiteralBytes.pb) is untouched, so IDs still
// round-trip byte-for-byte for evaluation and cache persistence. A consumer
// rebuilding a chain from telemetry keys frames by their carried digest, which
// the scrub preserves; only the raw argument bytes are withheld.
//
// When nothing needs scrubbing, callPB itself is returned; otherwise new
// messages are built only along changed paths, sharing unchanged subtrees.
func ScrubbedCallForTelemetry(callPB *callpbv1.Call) *callpbv1.Call {
	if callPB == nil {
		return nil
	}
	args, argsChanged := scrubbedTelemetryArguments(callPB.Args)
	inputs, inputsChanged := scrubbedTelemetryArguments(callPB.ImplicitInputs)
	if !argsChanged && !inputsChanged {
		return callPB
	}
	return &callpbv1.Call{
		ReceiverDigest: callPB.ReceiverDigest,
		Type:           callPB.Type,
		Field:          callPB.Field,
		Args:           args,
		Nth:            callPB.Nth,
		Module:         callPB.Module,
		Digest:         callPB.Digest,
		View:           callPB.View,
		EffectIds:      callPB.EffectIds,
		ExtraDigests:   callPB.ExtraDigests,
		ImplicitInputs: inputs,
	}
}

func scrubbedTelemetryArguments(args []*callpbv1.Argument) ([]*callpbv1.Argument, bool) {
	out := args
	changed := false
	for i, arg := range args {
		if arg == nil {
			continue
		}
		lit, litChanged := scrubbedTelemetryLiteral(arg.Value)
		if !litChanged {
			continue
		}
		if !changed {
			out = slices.Clone(args)
			changed = true
		}
		out[i] = &callpbv1.Argument{Name: arg.Name, Value: lit}
	}
	return out, changed
}

func scrubbedTelemetryLiteral(lit *callpbv1.Literal) (*callpbv1.Literal, bool) {
	if lit == nil {
		return lit, false
	}
	switch v := lit.Value.(type) {
	case *callpbv1.Literal_Bytes:
		if len(v.Bytes) <= MaxTelemetryBytesLiteral {
			return lit, false
		}
		return &callpbv1.Literal{
			Value: &callpbv1.Literal_String_{String_: DisplayBytes(v.Bytes)},
		}, true
	case *callpbv1.Literal_List:
		if v.List == nil {
			return lit, false
		}
		values, changed := scrubbedTelemetryLiterals(v.List.Values)
		if !changed {
			return lit, false
		}
		return &callpbv1.Literal{
			Value: &callpbv1.Literal_List{List: &callpbv1.List{Values: values}},
		}, true
	case *callpbv1.Literal_Object:
		if v.Object == nil {
			return lit, false
		}
		values, changed := scrubbedTelemetryArguments(v.Object.Values)
		if !changed {
			return lit, false
		}
		return &callpbv1.Literal{
			Value: &callpbv1.Literal_Object{Object: &callpbv1.Object{Values: values}},
		}, true
	default:
		return lit, false
	}
}

func scrubbedTelemetryLiterals(lits []*callpbv1.Literal) ([]*callpbv1.Literal, bool) {
	out := lits
	changed := false
	for i, lit := range lits {
		scrubbed, litChanged := scrubbedTelemetryLiteral(lit)
		if !litChanged {
			continue
		}
		if !changed {
			out = slices.Clone(lits)
			changed = true
		}
		out[i] = scrubbed
	}
	return out, changed
}
