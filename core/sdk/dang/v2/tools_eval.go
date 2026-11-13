package dangv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/vito/dang/v2/pkg/dang"
	"github.com/vito/dang/v2/pkg/dang/analysis"
	"github.com/vito/dang/v2/pkg/hm"
	"github.com/vito/dang/v2/pkg/ioctx"

	"github.com/dagger/dagger/core"
)

// This file is the Dang side of the DangEval builtin (see
// core/llm_dang_tools.go).

const (
	// dangEvalName names an evaluated snippet in positions and messages.
	dangEvalName = "<eval>"
	// dangOutputMax caps the printed output a DangEval report holds, only
	// to bound its memory: the tool call's logs have all of it, and the
	// engine bounds what the model sees of the report as it bounds a
	// module tool's output. Its tail is kept.
	dangOutputMax = 1 << 20
	// dangMemberNamesMax caps the member names listed after a result.
	dangMemberNamesMax = 80
	// dangBlockStringMin is how long (quoted) a multi-line string result
	// gets before it is shown as a block of lines rather than one quoted
	// line: the width Describe lays values out in.
	dangBlockStringMin = 80
)

// dangDescribeOptions describe a DangEval result in full, without
// eliding the middle of strings, the rest of lists or the end of the
// rendering: the engine bounds the report as it bounds a module tool's
// output, and the tool call's logs keep it whole. Depth stays capped:
// a value is described, not dumped.
var dangDescribeOptions = dang.DescribeOptions{MaxString: -1, MaxItems: -1, MaxBytes: -1}

// dangWrite is a bound value a snippet reassigned, with the ID of its final
// value (see core.DangWrite).
type dangWrite struct {
	name, typeName, id string
}

// dangWriter applies a successful snippet's writes and summarizes them.
type dangWriter func(context.Context, []dangWrite) (string, error)

// evalDang is DangEval: it checks source as one more file of the scope, and
// only if that succeeds loads the scope (a module's top level runs) and runs
// source in it, with the Dagger import's queries sent to client. A type error
// or a failure is reported like a result, as DangCheck reports diagnostics:
// the error return is for what keeps it from running at all.
//
// The whole report goes to output too, as a module function's prints go to
// its call's logs: printed output as it is printed, and the rest of the
// report once it is known. Secret plaintexts are scrubbed from both.
//
// Assignment is the write: when source succeeds, the scope's bound values
// (`ws`, the bound tool objects) it reassigned are handed to write with
// their final values. A snippet that fails writes nothing.
func evalDang(ctx context.Context, sc *dangScope, source string, client graphql.Client, output io.Writer, write dangWriter) (core.DangReport, error) {
	start := time.Now()
	var r dangReport
	calls := &countingClient{inner: client}
	if output == nil {
		output = io.Discard
	}
	logs := &lineEndWriter{w: output}
	// printedEnd is where the report's printed output ends: the rest is
	// logged when the report is done.
	printedEnd := 0
	done := func() core.DangReport {
		report := r.String()
		logs.WriteRest(calls.secrets.scrub(report[printedEnd:]))
		return core.DangReport{Text: calls.secrets.scrub(report)}
	}

	// Check first: a type error runs nothing.
	snippet := sc.snippetPath()
	a, err := analysis.Analyze(ctx, sc.source(map[string]string{snippet: source}), sc.dir, analysis.Options{
		Focus:             snippet,
		ImportConfigs:     sc.imports(),
		DiagnosticContext: true,
	})
	if err != nil {
		return core.DangReport{}, err
	}
	if diags := a.Diagnostics(); hasErrors(diags) {
		r.diagnostics(sc, diags)
		r.linef("%s · %s · nothing ran", r.counts(), sc.label)
		return done(), nil
	}

	ctx = dang.ContextWithImportConfigs(ctx, dang.ImportConfig{
		Name: "Dagger", Client: calls, Schema: sc.schema, AutoImport: true,
	})
	var printed tailBuffer
	sink := io.Writer(&scrubWriter{w: io.MultiWriter(&printed, logs), secrets: &calls.secrets})
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
			r.WriteString(out)
			if !strings.HasSuffix(out, "\n") {
				r.WriteByte('\n')
			}
		}
		printedEnd = r.Len()
	}
	fail := func(err error) core.DangReport {
		writeOutput()
		diags := errorDiagnostics(err)
		if len(diags) == 0 {
			diags = []analysis.Diagnostic{{Severity: analysis.SeverityError, Message: err.Error()}}
		}
		sc.attributeBindingFailures(diags)
		r.diagnostics(sc, diags)
		r.linef("%s", footer("failed"))
		return done()
	}

	mod, err := dang.LoadDir(ctx, sc.dir, dang.LoadOptions{FS: sc.source(nil), Eager: sc.module})
	if err != nil {
		return fail(err), nil
	}
	defer mod.Close()
	session, err := mod.NewSession(ctx)
	if err != nil {
		return fail(err), nil
	}
	ran, err := session.Check(ctx, dangEvalName, source)
	if err != nil {
		return fail(err), nil
	}
	val, err := ran.Eval(ctx)
	if err != nil {
		return fail(err), nil
	}
	writes, err := finalWrites(ctx, session, mod.Version, sc.bound, source)
	if err != nil {
		return fail(err), nil
	}

	writeOutput()
	wroteWS := slices.ContainsFunc(writes, func(w dangWrite) bool { return w.name == "ws" })
	unwritten := reportEvalResult(ctx, &r, calls, val, ran.Type, wroteWS)
	r.linef("%s", footer("ok"))
	switch {
	case len(writes) > 0 && write != nil:
		summary, err := write(ctx, writes)
		if err != nil {
			r.linef("write failed: %s", err)
		} else if summary != "" {
			r.linef("%s", summary)
		}
	case unwritten:
		// Only where it matters: a result that looks like a write.
		r.linef("0 writes")
	}
	return done(), nil
}

// finalWrites returns the bound values source reassigns, with their values
// as the session holds them after it ran. A value is written by assigning
// it (`ws = ws.withChanges(…)`, `editor = editor.todoWrite(…)`); calling a
// method on it, or returning a new one, writes nothing.
func finalWrites(ctx context.Context, session *dang.Session, version dang.Version, bound []dangValue, source string) ([]dangWrite, error) {
	assigned := reassignedNames(source, version)
	var writes []dangWrite
	for _, v := range bound {
		if !assigned[v.name] {
			continue
		}
		read, err := session.Check(ctx, "<"+v.name+">", v.name)
		if err != nil {
			return nil, err
		}
		final, err := read.Eval(ctx)
		if err != nil {
			return nil, err
		}
		g, ok := final.(dang.GraphQLValue)
		if !ok {
			return nil, fmt.Errorf("%s is a %T, not an API object", v.name, final)
		}
		id, err := g.ID(ctx)
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", v.name, err)
		}
		writes = append(writes, dangWrite{name: v.name, typeName: v.typeName, id: id})
	}
	return writes, nil
}

// reassignedNames are the names source assigns with `=`, anywhere in it.
func reassignedNames(source string, version dang.Version) map[string]bool {
	names := map[string]bool{}
	parsed, err := dang.Parse(dangEvalName, []byte(source), dang.WithVersion(version))
	if err != nil {
		return names
	}
	block, ok := parsed.(*dang.FileBlock)
	if !ok {
		return names
	}
	block.Walk(func(n dang.Node) bool {
		if n == nil {
			return false
		}
		if re, ok := n.(*dang.Reassignment); ok {
			if sym, ok := re.Target.(*dang.Symbol); ok {
				names[sym.Name] = true
			}
		}
		return true
	})
	return names
}

// reportEvalResult reports a snippet's result: its value and its type's
// members. A result is described, never applied: writing is assigning (see
// finalWrites), so a Workspace or Changeset the snippet only returns says
// how to write it; reportEvalResult reports whether it was one.
func reportEvalResult(ctx context.Context, r *dangReport, calls *countingClient, val dang.Value, typ hm.Type, wroteWS bool) (unwritten bool) {
	r.linef("%s", calls.secrets.scrub(describeResult(ctx, val, typ)))
	if typ == nil {
		return false
	}
	switch strings.TrimPrefix(strings.TrimSuffix(dang.FormatType(typ), "!"), "Dagger.") {
	case "Changeset":
		r.linef("(not applied; to write it, assign it: ws = ws.withChanges(…))")
		unwritten = true
	case "Workspace":
		if !wroteWS {
			r.linef("(not applied; to write it, assign it: ws = …)")
			unwritten = true
		}
	}
	members := analysis.TypeMembers(typ)
	if len(members) == 0 {
		return unwritten
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
	return unwritten
}

// describeResult describes a snippet's result as `TYPE = value`, in full
// (see dangDescribeOptions). A long multi-line string, such as a file's
// contents, is shown as a block of its own lines between triple quotes
// rather than as one quoted line, so it reads as the text it is, and a
// reader that bounds lines (the tool result, ReadLogs) cuts it by lines.
func describeResult(ctx context.Context, val dang.Value, typ hm.Type) string {
	if s, ok := val.(dang.StringValue); ok && isBlockString(s.Val) {
		if typ == nil {
			typ = s.Type()
		}
		text := s.Val
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return fmt.Sprintf("%s = \"\"\"\n%s\"\"\"", typ, text)
	}
	return dang.Describe(ctx, val, typ, dangDescribeOptions)
}

// isBlockString reports whether a string result is shown as a block: it
// spans lines, and quoted on one it would not fit dangBlockStringMin.
func isBlockString(s string) bool {
	return strings.Contains(strings.TrimSuffix(s, "\n"), "\n") && len(strconv.Quote(s)) > dangBlockStringMin
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

// lineEndWriter writes to w, remembering whether what it wrote last ended
// a line, so the rest of a report logged after printed output starts on a
// line of its own, as it does in the report.
type lineEndWriter struct {
	mu      sync.Mutex
	w       io.Writer
	midLine bool
}

func (l *lineEndWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(p) > 0 {
		l.midLine = p[len(p)-1] != '\n'
	}
	return l.w.Write(p)
}

// WriteRest writes the rest of a report, on a line of its own. Logging is
// best effort: a failure to log doesn't fail the call.
func (l *lineEndWriter) WriteRest(rest string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if rest == "" {
		return
	}
	if l.midLine {
		rest = "\n" + rest
	}
	_, _ = io.WriteString(l.w, rest)
	l.midLine = !strings.HasSuffix(rest, "\n")
}
