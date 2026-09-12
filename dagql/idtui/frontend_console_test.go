package idtui

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dagger/dagger/dagql/dagui"
)

func TestValidateConsoleKey(t *testing.T) {
	valid := []string{
		// named keys
		"enter", "esc", "escape", "tab", "space", "backspace",
		"up", "down", "left", "right", "home", "end", "pgup", "pgdown",
		"insert", "delete", "begin", "find", "select",
		// bare characters (including the keymap's own bindings)
		"a", "L", "T", "/", "-", "?",
		// the plus key and modified pluses
		"+", "ctrl++",
		// f-keys
		"f1", "f10", "f20",
		// modifier combos
		"ctrl+c", "ctrl+s", "alt+enter", "shift+tab", "ctrl+alt+delete",
		"meta+x", "super+z", "hyper+q",
	}
	for _, spec := range valid {
		if err := validateConsoleKey(spec); err != nil {
			t.Errorf("validateConsoleKey(%q) = %v, want nil", spec, err)
		}
	}

	invalid := []string{
		// the emacs/tmux-style names that motivated validation: tuist would
		// type these into the TUI as literal text
		"C-s", "C-c", "M-x",
		// typos and unknown names
		"entr", "escpe", "control+c", "ctl+c",
		// a modifier with nothing after it is parsed as a key name
		"ctrl+notakey",
	}
	for _, spec := range invalid {
		err := validateConsoleKey(spec)
		if err == nil {
			t.Errorf("validateConsoleKey(%q) = nil, want error", spec)
			continue
		}
		if !strings.Contains(err.Error(), spec) {
			t.Errorf("validateConsoleKey(%q) error does not name the token: %v", spec, err)
		}
	}

	// A trailing bare modifier is a single-part spec, so it's a key *name*
	// lookup — "ctrl" alone is not a key.
	if err := validateConsoleKey("ctrl"); err == nil {
		t.Error("validateConsoleKey(\"ctrl\") = nil, want error")
	}

	// Repeat syntax is stripped by parseConsoleKeys before validation; a
	// malformed count survives as part of the token and must be rejected.
	keys := parseConsoleKeys("down*3 enter")
	if len(keys) != 4 {
		t.Fatalf("parseConsoleKeys(\"down*3 enter\") = %v", keys)
	}
	for _, k := range keys {
		if err := validateConsoleKey(k); err != nil {
			t.Errorf("validateConsoleKey(%q) = %v, want nil", k, err)
		}
	}
	for _, k := range parseConsoleKeys("down*x") {
		if err := validateConsoleKey(k); err == nil {
			t.Errorf("validateConsoleKey(%q) = nil, want error", k)
		}
	}
}

func TestConsoleDuration(t *testing.T) {
	for _, tc := range []struct {
		in   string
		def  time.Duration
		want time.Duration
		err  bool
	}{
		{in: "", def: 2 * time.Second, want: 2 * time.Second},
		{in: "5s", want: 5 * time.Second},
		{in: "1500ms", want: 1500 * time.Millisecond},
		{in: "30", want: 30 * time.Second},
		{in: "2.5", want: 2500 * time.Millisecond},
		{in: "bogus", err: true},
		{in: "-5s", err: true},
		{in: "-3", err: true},
	} {
		got, err := consoleDuration(tc.in, tc.def)
		if tc.err {
			if err == nil {
				t.Errorf("consoleDuration(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("consoleDuration(%q) = %v, want %v", tc.in, err, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("consoleDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestConsoleSpanDetail(t *testing.T) {
	db := dagui.NewDB()
	rootID := prettyTestSpanID(1)
	midID := prettyTestSpanID(2)
	leafID := prettyTestSpanID(3)
	start := time.Unix(100, 0).UTC()
	db.ImportSnapshots([]dagui.SpanSnapshot{
		{
			ID:        rootID,
			TraceID:   prettyTestTraceID(),
			Name:      "root call",
			StartTime: start,
			EndTime:   start.Add(3 * time.Second),
			Final:     true,
		},
		{
			ID:          midID,
			TraceID:     prettyTestTraceID(),
			Name:        "middle span",
			StartTime:   start.Add(time.Second),
			EndTime:     start.Add(3 * time.Second),
			ParentID:    rootID,
			Passthrough: true,
			RollUpLogs:  true,
			Final:       true,
		},
		{
			ID:           leafID,
			TraceID:      prettyTestTraceID(),
			Name:         "leaf op",
			StartTime:    start.Add(2 * time.Second),
			EndTime:      start.Add(3 * time.Second),
			ParentID:     midID,
			Internal:     true,
			Encapsulated: true,
			Cached:       true,
			Final:        true,
		},
	})
	db.SetPrimarySpan(rootID)
	fe := NewWithDB(io.Discard, db)

	detail, ok := fe.consoleSpanDetail(leafID)
	if !ok {
		t.Fatalf("consoleSpanDetail(%s) = not found", leafID)
	}
	for _, want := range []string{
		"span:     " + leafID.String() + "  leaf op",
		"status:   ok",
		"started:  " + start.Add(2*time.Second).Format(time.RFC3339Nano),
		"ended:    " + start.Add(3*time.Second).Format(time.RFC3339Nano),
		"duration: 1s",
		"flags:    internal encapsulated cached",
		"parents (nearest first):",
		"  " + midID.String() + "  middle span  [passthrough rollUpLogs]",
		"  " + rootID.String() + "  root call",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("span detail missing %q:\n%s", want, detail)
		}
	}
	// The flagless root ancestor must not grow an empty flag bracket.
	if strings.Contains(detail, "root call  [") {
		t.Errorf("root ancestor line should have no flag bracket:\n%s", detail)
	}

	// A root span reports its (lack of a) parent chain explicitly.
	rootDetail, ok := fe.consoleSpanDetail(rootID)
	if !ok {
		t.Fatalf("consoleSpanDetail(%s) = not found", rootID)
	}
	if !strings.Contains(rootDetail, "(none — root span)") {
		t.Errorf("root detail missing empty-parent marker:\n%s", rootDetail)
	}
	if !strings.Contains(rootDetail, "flags:    (none)") {
		t.Errorf("root detail missing empty flags marker:\n%s", rootDetail)
	}

	// Unknown spans are reported as such, not as an empty page.
	if _, ok := fe.consoleSpanDetail(prettyTestSpanID(99)); ok {
		t.Error("consoleSpanDetail of unknown span reported ok")
	}
}
