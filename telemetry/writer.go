package telemetry

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dagger/dagger/dagql/idproto"
	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock"
	"google.golang.org/protobuf/proto"
)

type writer struct {
	telemetry *Telemetry
	parenter  *Parenter

	// emitted tracks whether we've emitted a SpanPayload for a vertex yet
	emitted map[vertexParent]bool

	// calls stores the calls associated to spans
	calls        map[string]digest.Digest
	emittedCalls map[digest.Digest]bool

	mu sync.Mutex
}

type vertexParent struct {
	vertexID string
	parentID string
}

func NewWriter(t *Telemetry) progrock.Writer {
	return &writer{
		telemetry:    t,
		parenter:     NewParenter(),
		emitted:      map[vertexParent]bool{},
		calls:        map[string]digest.Digest{},
		emittedCalls: map[digest.Digest]bool{},
	}
}

func (t *writer) WriteStatus(ev *progrock.StatusUpdate) error {
	t.parenter.TrackUpdate(ev)

	t.mu.Lock()
	defer t.mu.Unlock()

	ts := time.Now().UTC()

	for _, m := range ev.Metas {
		if m.Name != "id" {
			continue
		}
		callID := digest.FromBytes(m.Data.Value)
		t.calls[m.Vertex] = callID
		t.maybeEmitCall(ts, callID, m.Data.Value)
	}

	for _, m := range ev.Children {
		for _, vid := range m.Vertexes {
			if v, found := t.parenter.Vertex(vid); found {
				t.maybeEmitOp(ts, v, false)
			}
		}
	}

	for _, eventVertex := range ev.Vertexes {
		if v, found := t.parenter.Vertex(eventVertex.Id); found {
			// override the vertex with the current event vertex since a
			// single PipelineEvent could contain duplicated vertices with
			// different data like started and completed
			v.Vertex = eventVertex
			t.maybeEmitOp(ts, v, true)
		}
	}

	for _, l := range ev.Logs {
		t.telemetry.Push(LogPayload{
			SpanID: l.Vertex,
			Data:   string(l.Data),
			Stream: int(l.Stream.Number()),
		}, l.Timestamp.AsTime())
	}

	return nil
}

func (t *writer) Close() error {
	t.telemetry.Close()
	return nil
}

// maybeEmitOp emits a OpPayload for a vertex if either A) an OpPayload hasn't
// been emitted yet because we saw the vertex before its membership, or B) the
// vertex has been updated.
func (t *writer) maybeEmitOp(ts time.Time, v *VertexWithParent, isUpdated bool) {
	key := vertexParent{
		vertexID: v.Id,
		parentID: v.Parent,
	}
	if t.emitted[key] && !isUpdated {
		return
	}
	t.telemetry.Push(t.vertexOp(v.Vertex, key.parentID), ts)
	t.emitted[key] = true
}

// maybeEmitOp emits a OpPayload for a vertex if either A) an OpPayload hasn't
// been emitted yet because we saw the vertex before its membership, or B) the
// vertex has been updated.
func (t *writer) maybeEmitCall(ts time.Time, id digest.Digest, buf []byte) {
	if t.emittedCalls[id] {
		return
	}
	t.telemetry.Push(t.callOp(id, buf), ts)
	t.emittedCalls[id] = true
}

func (t *writer) callOp(id digest.Digest, buf []byte) CallPayload {
	var call idproto.ID
	if err := proto.Unmarshal(buf, &call); err != nil {
		slog.Warn("failed to unmarshal call", "err", err)
	}

	op := CallPayload{
		ID: id.String(),

		ProtobufPayload: buf,
		ReturnType:      call.Type,
		Function:        call.Field,

		Tainted: call.Tainted,
		Nth:     call.Nth,
	}

	for _, arg := range call.Args {
		callArg := CallArg{
			Name: arg.Name,
		}
		lit, err := toCallLiteral(arg.Value)
		if err != nil {
			slog.Warn("failed to convert literal", "err", err)
			continue
		}
		callArg.Value = lit
		op.Args = append(op.Args, callArg)
	}

	if mod := call.Module; mod != nil {
		op.ModuleName = mod.Name
		op.ModuleRef = mod.Ref
		consID, err := mod.Id.Digest()
		if err != nil {
			slog.Warn("failed to digest module id", "id", mod.Id.Display(), "err", err)
		}
		op.ModuleConstructorID = consID.String()
	}

	return op
}

func toCallLiteral(lit *idproto.Literal) (CallLiteral, error) {
	callLit := CallLiteral{}
	switch val := lit.GetValue().(type) {
	case *idproto.Literal_Id:
		dig, err := val.Id.Digest()
		if err != nil {
			return CallLiteral{}, err
		}
		id := dig.String()
		callLit.ID = &id
	case *idproto.Literal_Null:
		null := true
		callLit.Null = &null
	case *idproto.Literal_Bool:
		callLit.Bool = &val.Bool
	case *idproto.Literal_Int:
		callLit.Int = &val.Int
	case *idproto.Literal_Float:
		callLit.Float = &val.Float
	case *idproto.Literal_Enum:
		callLit.Enum = &val.Enum
	case *idproto.Literal_String_:
		callLit.String = &val.String_
	case *idproto.Literal_List:
		list := make([]CallLiteral, len(val.List.Values))
		for i, l := range val.List.Values {
			elemLit, err := toCallLiteral(l)
			if err != nil {
				return CallLiteral{}, fmt.Errorf("[%d]: %w", i, err)
			}
			list[i] = elemLit
		}
		callLit.List = &list
	case *idproto.Literal_Object:
		obj := make([]CallArg, len(val.Object.Values))
		for i, a := range val.Object.Values {
			valLit, err := toCallLiteral(a.Value)
			if err != nil {
				return CallLiteral{}, fmt.Errorf("[%d]: %w", i, err)
			}
			obj[i] = CallArg{
				Name:  a.Name,
				Value: valLit,
			}
		}
		callLit.Object = &obj
	default:
		panic("unreachable")
	}
	return callLit, nil
}

func (t *writer) vertexOp(v *progrock.Vertex, parentID string) SpanPayload {
	op := SpanPayload{
		ID:       v.Id,
		ParentID: parentID,

		Name:     v.Name,
		Internal: v.Internal,

		Cached: v.Cached,
		Error:  v.GetError(),

		Inputs: v.Inputs,
	}

	if len(v.Outputs) > 0 {
		op.ResultCallID = v.Outputs[0]
	}

	if v.Started != nil {
		t := v.Started.AsTime()
		op.Started = &t
	}

	if v.Completed != nil {
		t := v.Completed.AsTime()
		op.Completed = &t
	}

	return op
}
