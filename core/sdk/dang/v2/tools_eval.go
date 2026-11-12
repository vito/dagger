package dangv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/vito/dang/v2/pkg/dang"
	"github.com/vito/dang/v2/pkg/dang/analysis"
	"github.com/vito/dang/v2/pkg/hm"
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
// output goes to output as well as into the report. A type error or a
// failure is reported like a result, as DangCheck reports diagnostics: the
// error return is for what keeps it from running at all.
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
		return r.String(), nil
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
		return calls.secrets.scrub(r.String()), nil
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
	reportEvalResult(ctx, &r, calls, val, typ)
	r.linef("%s", footer("ok"))
	return r.String(), nil
}

// reportEvalResult reports a snippet's result: its value and its type's
// members. This is where the result of an eval is produced, and all that
// is done with it: it is described, never applied. In particular a
// Changeset or a bound tool object's new state the snippet returns changes
// neither the workspace nor the bindings; acting on a result (as a tool
// call's is, MCP.routeObjectMethodResult) would start here.
func reportEvalResult(ctx context.Context, r *dangReport, calls *countingClient, val dang.Value, typ hm.Type) {
	r.linef("%s", calls.secrets.scrub(dang.Describe(ctx, val, typ, dang.DescribeOptions{})))
	if typ == nil {
		return
	}
	members := analysis.TypeMembers(typ)
	if len(members) == 0 {
		return
	}
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
