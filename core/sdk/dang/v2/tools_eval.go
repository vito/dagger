package dangv2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/vito/dang/v2/pkg/dang"
	"github.com/vito/dang/v2/pkg/dang/analysis"
	"github.com/vito/dang/v2/pkg/ioctx"
)

// This file is the Dang side of the DangEval builtin (see
// core/llm_dang_tools.go).

const (
	// dangEvalName names an evaluated snippet in positions and messages.
	dangEvalName = "<eval>"
	// dangOutputMax caps the printed output a DangEval result shows; its
	// tail is kept, where failures and results tend to be.
	dangOutputMax = 8 << 10
	// dangMemberNamesMax caps the member names listed after a result.
	dangMemberNamesMax = 80
)

// evalDang is DangEval: it checks source as one more file of the scope, and
// only if that succeeds loads the scope (a module's top level runs) and runs
// source in it, with the Dagger import's queries sent to client. Printed
// output goes to output as well as into the result. A type error or a
// failure is returned as an error carrying the report.
func evalDang(ctx context.Context, sc *dangScope, source string, client graphql.Client, output io.Writer) (string, error) {
	start := time.Now()
	var r dangReport

	// Check first: a type error runs nothing.
	snippet := sc.snippetPath()
	a, err := analysis.Analyze(ctx, sc.source(map[string]string{snippet: source}), sc.dir, analysis.Options{
		Focus:         snippet,
		ImportConfigs: sc.imports(),
	})
	if err != nil {
		return "", err
	}
	if diags := a.Diagnostics(); hasErrors(diags) {
		r.diagnostics(sc, diags)
		r.linef("%s · %s · nothing ran", r.counts(), sc.label)
		return "", errors.New(r.String())
	}

	calls := &countingClient{inner: client}
	ctx = dang.ContextWithImportConfigs(ctx, dang.ImportConfig{
		Name: "Dagger", Client: calls, Schema: sc.schema, AutoImport: true,
	})
	var printed tailBuffer
	sink := io.Writer(&printed)
	if output != nil {
		sink = io.MultiWriter(&printed, output)
	}
	sink = &scrubWriter{w: sink, secrets: &calls.secrets}
	ctx = ioctx.StdoutToContext(ctx, sink)
	ctx = ioctx.StderrToContext(ctx, sink)

	footer := func(status string) string {
		return strings.Join([]string{
			status,
			formatDuration(time.Since(start)),
			plural(int(calls.count.Load()), "engine call"),
			sc.label,
		}, " · ")
	}
	writeOutput := func() {
		if out := printed.String(); out != "" {
			r.linef("output:")
			r.WriteString(calls.secrets.scrub(out))
			if !strings.HasSuffix(out, "\n") {
				r.WriteByte('\n')
			}
		}
	}
	fail := func(err error) (string, error) {
		writeOutput()
		diags := errorDiagnostics(err)
		if len(diags) == 0 {
			diags = []analysis.Diagnostic{{Severity: analysis.SeverityError, Message: err.Error()}}
		}
		r.diagnostics(sc, diags)
		r.linef("%s", footer("failed"))
		return "", errors.New(calls.secrets.scrub(r.String()))
	}

	mod, err := dang.LoadDir(ctx, sc.dir, dang.LoadOptions{FS: sc.source(nil), Eager: sc.module})
	if err != nil {
		return fail(err)
	}
	defer mod.Close()
	val, typ, err := mod.Eval(ctx, dangEvalName, source)
	if err != nil {
		return fail(err)
	}

	writeOutput()
	r.linef("%s", calls.secrets.scrub(dang.Describe(ctx, val, typ, dang.DescribeOptions{})))
	if typ != nil {
		if members := analysis.TypeMembers(typ); len(members) > 0 {
			names := make([]string, 0, len(members))
			for i, m := range members {
				if i == dangMemberNamesMax {
					names = append(names, fmt.Sprintf("… (%d more)", len(members)-i))
					break
				}
				names = append(names, m.Name)
			}
			r.linef("members: %s", strings.Join(names, " "))
		}
	}
	r.linef("%s", footer("ok"))
	return r.String(), nil
}

func hasErrors(diags []analysis.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == analysis.SeverityError {
			return true
		}
	}
	return false
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return d.Round(100 * time.Millisecond).String()
	}
}

// countingClient counts the queries a snippet sends, and remembers the
// secret plaintexts they return, to scrub them from what is shown.
type countingClient struct {
	inner   graphql.Client
	count   atomic.Int64
	secrets secretSet
}

func (c *countingClient) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	c.count.Add(1)
	err := c.inner.MakeRequest(ctx, req, resp)
	if err == nil && req != nil && resp != nil && strings.Contains(req.Query, "plaintext") {
		// Secret.plaintext is the one way a snippet reads a secret: every
		// string such a query returns is treated as one.
		if data, err := json.Marshal(resp.Data); err == nil {
			var decoded any
			if json.Unmarshal(data, &decoded) == nil {
				c.secrets.addStrings(decoded)
			}
		}
	}
	return err
}

// secretSet holds the secret plaintexts seen, to scrub from output.
type secretSet struct {
	mu     sync.Mutex
	values []string
}

func (s *secretSet) addStrings(v any) {
	switch v := v.(type) {
	case string:
		if v != "" {
			s.mu.Lock()
			s.values = append(s.values, v)
			s.mu.Unlock()
		}
	case []any:
		for _, e := range v {
			s.addStrings(e)
		}
	case map[string]any:
		for _, e := range v {
			s.addStrings(e)
		}
	}
}

func (s *secretSet) scrub(text string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, secret := range s.values {
		text = strings.ReplaceAll(text, secret, "***")
	}
	return text
}

// scrubWriter scrubs secrets from what it writes.
type scrubWriter struct {
	w       io.Writer
	secrets *secretSet
}

func (s *scrubWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(s.w, s.secrets.scrub(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// tailBuffer keeps the last dangOutputMax bytes written to it.
type tailBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	dropped int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if over := b.buf.Len() - dangOutputMax; over > 0 {
		b.dropped += over
		b.buf.Next(over)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.dropped > 0 {
		return fmt.Sprintf("… (%d earlier bytes cut)\n%s", b.dropped, b.buf.String())
	}
	return b.buf.String()
}
