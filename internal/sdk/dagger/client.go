//go:generate dagger client-gen -o api.gen.go
package dagger

import (
	"context"
	"net"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/dagger/dagger/engine"
	"github.com/dagger/dagger/internal/querybuilder"
	"github.com/dagger/dagger/router"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Client is the Dagger Engine Client
type Client struct {
	Query

	stop   func()
	doneCh chan error

	gql graphql.Client
}

// Connect to a Dagger Engine
func Connect(ctx context.Context, optionalCfg ...*engine.Config) (_ *Client, rerr error) {
	ctx, stop := context.WithCancel(ctx)

	c := &Client{
		stop:   stop,
		doneCh: make(chan error, 1),
	}

	var config *engine.Config
	if len(optionalCfg) > 0 {
		config = optionalCfg[0]
	}

	started := make(chan struct{})
	var client *http.Client
	go func() {
		defer close(c.doneCh)
		c.doneCh <- engine.Start(ctx, config, func(ctx context.Context, r *router.Router) error {
			client = &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						// TODO: not efficient, but whatever
						serverConn, clientConn := net.Pipe()
						go r.ServeConn(serverConn)

						return clientConn, nil
					},
				},
			}
			close(started)
			<-ctx.Done()
			return nil
		})
	}()

	select {
	case <-started:
	case err := <-c.doneCh:
		return nil, err
	}

	c.gql = graphql.NewClient("http://dagger/query", client)
	c.Query = Query{
		q: querybuilder.Query(),
		c: c.gql,
	}

	return c, nil
}

// Close the engine connection
func (c *Client) Close() error {
	c.stop()
	return <-c.doneCh
}

// Do sends a GraphQL request to the engine
func (c *Client) Do(ctx context.Context, req *Request, resp *Response) error {
	r := graphql.Response{}
	if resp != nil {
		r.Data = resp.Data
		r.Errors = resp.Errors
		r.Extensions = resp.Extensions
	}
	return c.gql.MakeRequest(ctx, &graphql.Request{
		Query:     req.Query,
		Variables: req.Variables,
		OpName:    req.OpName,
	}, &r)
}

// Request contains all the values required to build queries executed by
// the graphql.Client.
//
// Typically, GraphQL APIs will accept a JSON payload of the form
//
//	{"query": "query myQuery { ... }", "variables": {...}}`
//
// and Request marshals to this format.  However, MakeRequest may
// marshal the data in some other way desired by the backend.
type Request struct {
	// The literal string representing the GraphQL query, e.g.
	// `query myQuery { myField }`.
	Query string `json:"query"`
	// A JSON-marshalable value containing the variables to be sent
	// along with the query, or nil if there are none.
	Variables interface{} `json:"variables,omitempty"`
	// The GraphQL operation name. The server typically doesn't
	// require this unless there are multiple queries in the
	// document, but genqlient sets it unconditionally anyway.
	OpName string `json:"operationName"`
}

// Response that contains data returned by the GraphQL API.
//
// Typically, GraphQL APIs will return a JSON payload of the form
//
//	{"data": {...}, "errors": {...}}
//
// It may additionally contain a key named "extensions", that
// might hold GraphQL protocol extensions. Extensions and Errors
// are optional, depending on the values returned by the server.
type Response struct {
	Data       interface{}            `json:"data"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	Errors     gqlerror.List          `json:"errors,omitempty"`
}
