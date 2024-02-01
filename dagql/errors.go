package dagql

import "github.com/vektah/gqlparser/v2/ast"

type MissingSubselectionsError struct {
	// Path is the path to the field that is missing subselections.
	Path ast.Path
}
