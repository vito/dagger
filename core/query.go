package core

import (
	"github.com/dagger/dagger/core/pipeline"
)

type Query struct {
	// Query itself is ID-able.
	//
	// This is slightly mindblowing, but it allows the ID to be constructed
	// before we get to the point where we create another type. And it's
	// technically correct anyway; evaluating the ID will yield the query in the
	// same state.
	IDable

	// Pipeline
	Pipeline pipeline.Path `json:"pipeline"`
}

func (query *Query) PipelinePath() pipeline.Path {
	if query == nil {
		return nil
	}
	return query.Pipeline
}
