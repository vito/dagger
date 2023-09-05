package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"strings"

	"dagger.io/dagger"
	"github.com/moby/buildkit/identity"
)

func main() {
	ctx := context.Background()

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	depthStr, fetchURL := os.Args[1], os.Args[2]

	depth, err := strconv.Atoi(depthStr)
	if err != nil {
		fatal(err)
	}

	out, err := fetch(ctx, c, fetchURL)
	if err != nil {
		fatal(err)
	}

	if depth > 1 {
		out, err = weHaveToGoDeeper(ctx, c, depth, out)
		if err != nil {
			fatal(err)
		}
	}

	fmt.Print(out)
}

func fatal(err any) {
	fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m\n", err)
	os.Exit(1)
}

func weHaveToGoDeeper(ctx context.Context, c *dagger.Client, depth int, lastOut string) (string, error) {
	code := c.Host().Directory(".", dagger.HostDirectoryOpts{
		Include: []string{"core/integration/testdata/nested-c2h/", "sdk/go/", "go.mod", "go.sum"},
	})

	l, port, err := localService(strconv.Itoa(depth - 1))
	if err != nil {
		return "", err
	}

	defer l.Close()

	host := c.Host().Service([]dagger.PortForward{
		{Frontend: port, Backend: port},
	})

	return c.Container().
		From("golang:1.20.6-alpine").
		WithMountedCache("/go/pkg/mod", c.CacheVolume("go-mod")).
		WithEnvVariable("GOMODCACHE", "/go/pkg/mod").
		WithMountedCache("/go/build-cache", c.CacheVolume("go-build")).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithServiceBinding("www", host).
		WithMountedDirectory("/src", code).
		WithWorkdir("/src").
		WithEnvVariable("NOW", identity.NewID()).
		WithExec([]string{
			"go", "run", "./core/integration/testdata/nested-c2h/",
			strconv.Itoa(depth - 1), "http://www:" + strconv.Itoa(port) + "/?content=" + lastOut,
		}, dagger.ContainerWithExecOpts{
			ExperimentalPrivilegedNesting: true,
		}).
		Stdout(ctx)
}

func fetch(ctx context.Context, c *dagger.Client, fetchURL string) (string, error) {
	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return "", err
	}
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			log.Println("!!! GOT CONN:", connInfo.Conn.LocalAddr())
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// NB: duplicated with core/integration/services_test.go
func localService(prefix string) (net.Listener, int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}

	go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, prefix+"-"+r.URL.Query().Get("content"))
	}))

	_, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, 0, err
	}

	return l, port, nil
}
