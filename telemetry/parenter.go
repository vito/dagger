package telemetry

import (
	"sync"

	"github.com/vito/progrock"
)

// Parenter vertex information to parent/child information emitted separately
// because we don't have access to the parent at the time that we convert
// Buildkit statuses to Progrock updates.
type Parenter struct {
	mu sync.Mutex

	closed bool

	// parents stores the parent of each span
	parents map[string]string

	// vertices stores and updates vertexes as they received, so that they can
	// be associated to pipeline paths once their membership is known.
	vertices map[string]*progrock.Vertex
}

func NewParenter() *Parenter {
	return &Parenter{
		parents:  map[string]string{},
		vertices: map[string]*progrock.Vertex{},
	}
}

// VertexWithParent is a Progrock vertex paired with all of its pipeline paths.
type VertexWithParent struct {
	*progrock.Vertex

	Parent string
}

func (t *Parenter) TrackUpdate(ev *progrock.StatusUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}

	for _, m := range ev.Children {
		for _, vid := range m.Vertexes {
			t.parents[vid] = m.Vertex
		}
	}

	for _, v := range ev.Vertexes {
		t.vertices[v.Id] = v
	}
}

func (t *Parenter) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return nil
}

func (t *Parenter) Vertex(id string) (*VertexWithParent, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	v, found := t.vertices[id]
	if !found {
		return nil, false
	}

	return t.vertex(v), true
}

func (t *Parenter) Vertices() []*VertexWithParent {
	t.mu.Lock()
	defer t.mu.Unlock()

	vertices := make([]*VertexWithParent, 0, len(t.vertices))
	for _, v := range t.vertices {
		vertices = append(vertices, t.vertex(v))
	}
	return vertices
}

func (t *Parenter) vertex(v *progrock.Vertex) *VertexWithParent {
	return &VertexWithParent{
		Vertex: v,
		Parent: t.parents[v.Id],
	}
}
