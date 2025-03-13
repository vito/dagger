package bbi

import (
	"context"
	"fmt"

	"github.com/dagger/dagger/dagql"
)

// Start a new BBI session
func NewSession(driver string, self dagql.Object, srv *dagql.Server) (Session, error) {
	drv, ok := drivers[driver]
	if !ok {
		return nil, fmt.Errorf("no such driver: %s", driver)
	}
	return drv.NewSession(self, srv), nil
}

// BBI stands for "Body-Brain Interface".
// A BBI implements a strategy for mapping a Dagger object's API to LLM function calls
// The perfect BBI has not yet been designed, so there are multiple BBI implementations,
// and an interface for easily swapping them out.
// Hopefully in the future the perfect BBI design will emerge, and we can retire
// the pluggable interface.
type Driver interface {
	NewSession(dagql.Object, *dagql.Server) Session
}

var drivers = make(map[string]Driver)

func Register(name string, driver Driver) {
	drivers[name] = driver
}

// A stateful BBI session
type Session interface {
	// Return a set of tools for the next llm loop
	// The tools may modify the state without worrying about synchronization:
	// it's the agent's responsibility to not call tools concurrently.
	Tools() []Tool
	Self() dagql.Object
}

// A frontend for LLM tool calling
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Call        func(context.Context, any) (any, error)
}
