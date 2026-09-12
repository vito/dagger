package call

import "github.com/dagger/dagger/dagql/call/callpbv1"

// MaxSpanAttrBytesLiteral is the largest raw Bytes literal a call frame may
// carry and still ride the legacy dagger.io/dag.call span attribute.
//
// Call payloads themselves deliberately carry raw bytes: they are the
// material for rebuilding — and resuming from — a trace, so the log channel
// (the modern payload transport) always delivers frames verbatim. The span
// attribute is a compatibility duplicate for older clients, and it is the
// wrong place for bulk data twice over: a multi-megabyte attribute rides
// every export of the span, and consumers fold both channels into a
// first-copy-wins store (dagui's DB.addCall), so a span-borne copy that
// diverged from the log copy could permanently shadow it. Frames carrying
// bytes beyond this limit therefore skip the attribute entirely and rely on
// the log channel alone.
const MaxSpanAttrBytesLiteral = 1 << 10 // 1 KiB

// HasOversizedBytesLiterals reports whether callPB carries a Bytes literal
// larger than MaxSpanAttrBytesLiteral anywhere in its arguments or implicit
// inputs, nested list and object literals included.
func HasOversizedBytesLiterals(callPB *callpbv1.Call) bool {
	if callPB == nil {
		return false
	}
	return argumentsHaveOversizedBytes(callPB.Args) ||
		argumentsHaveOversizedBytes(callPB.ImplicitInputs)
}

func argumentsHaveOversizedBytes(args []*callpbv1.Argument) bool {
	for _, arg := range args {
		if arg != nil && literalHasOversizedBytes(arg.Value) {
			return true
		}
	}
	return false
}

func literalHasOversizedBytes(lit *callpbv1.Literal) bool {
	if lit == nil {
		return false
	}
	switch v := lit.Value.(type) {
	case *callpbv1.Literal_Bytes:
		return len(v.Bytes) > MaxSpanAttrBytesLiteral
	case *callpbv1.Literal_List:
		if v.List == nil {
			return false
		}
		for _, elem := range v.List.Values {
			if literalHasOversizedBytes(elem) {
				return true
			}
		}
		return false
	case *callpbv1.Literal_Object:
		if v.Object == nil {
			return false
		}
		return argumentsHaveOversizedBytes(v.Object.Values)
	default:
		return false
	}
}
