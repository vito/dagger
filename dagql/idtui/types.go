package idtui

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strings"

	"github.com/a-h/templ"
	svg "github.com/ajstarks/svgo"
	"github.com/dagger/dagger/dagql/idproto"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/vito/invaders"
	"github.com/vito/midterm"
	"github.com/vito/progrock"
)

type State struct {
	Tape     *progrock.Tape
	Frontend *DB
}

func NewState(tape *progrock.Tape, fe *DB) *State {
	return &State{
		Tape:     tape,
		Frontend: fe,
	}
}

func CollectSteps(db *DB) []*Step {
	var steps []*Step // nolint:prealloc
	for vID := range db.Vertices {
		step, ok := db.Step(vID)
		if !ok {
			continue
		}
		steps = append(steps, step)
	}
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].IsBefore(steps[j])
	})
	return steps
}

func CollectRows(steps []*Step) []*TraceRow {
	var rows []*TraceRow
	WalkSteps(steps, func(row *TraceRow) {
		if row.Parent != nil {
			row.Parent.Children = append(row.Parent.Children, row)
		} else {
			rows = append(rows, row)
		}
	})
	return rows
}

type Pipeline []TraceRow

type TraceRow struct {
	Step *Step

	Parent   *TraceRow
	ByParent bool

	Children []*TraceRow
}

func (row *TraceRow) Depth() int {
	if row.Parent == nil {
		return 0
	}
	return row.Parent.Depth() + 1
}

func WalkSteps(steps []*Step, f func(*TraceRow)) {
	var lastSeen string
	seen := map[string]bool{}
	var walk func(*Step, *TraceRow)
	walk = func(step *Step, parent *TraceRow) {
		if seen[step.Digest] {
			return
		}
		row := &TraceRow{
			Step:   step,
			Parent: parent,
		}
		if step.Parent != nil {
			row.ByParent = step.Parent.Digest == lastSeen
		}
		f(row)
		lastSeen = step.Digest
		seen[step.Digest] = true
		for _, child := range step.Children() {
			walk(child, row)
		}
		lastSeen = step.Digest
	}
	for _, step := range steps {
		walk(step, nil)
	}
}

func idDigest(id *idproto.ID) string {
	dig, err := id.Digest()
	if err != nil {
		panic(err)
	}
	return dig.String()
}

func Invader(dig string) templ.Component {
	h := fnv.New64a()
	if _, err := h.Write([]byte(dig)); err != nil {
		panic(err)
	}

	invader := &invaders.Invader{}
	invader.Set(rand.New(rand.NewSource(int64(h.Sum64()))))

	avatarSvg := new(bytes.Buffer)
	canvas := svg.New(avatarSvg)

	cellSize := 9
	canvas.Startview(
		cellSize*invaders.Width,
		cellSize*invaders.Height,
		0,
		0,
		cellSize*invaders.Width,
		cellSize*invaders.Height,
	)
	canvas.Group()

	for row := range invader {
		y := row * cellSize

		for col := range invader[row] {
			x := col * cellSize
			shade := invader[row][col]

			var color string
			switch shade {
			case invaders.Background:
				color = "transparent"
			case invaders.Shade1:
				color = "#E43550"
			case invaders.Shade2:
				color = "#F08328"
			case invaders.Shade3:
				color = "#FCC51D"
			case invaders.Shade4:
				color = "#4DCC7D"
			case invaders.Shade5:
				color = "#47CED1"
			case invaders.Shade6:
				color = "#1D59FE"
			case invaders.Shade7:
				color = "#3FBDDD"
			default:
				panic(fmt.Errorf("invalid shade: %v", shade))
			}

			canvas.Rect(
				x, y,
				cellSize, cellSize,
				fmt.Sprintf("fill: %s", color),
				`shape-rendering="crispEdges"`,
			)
		}
	}

	canvas.Gend()
	canvas.End()

	return templ.Raw(avatarSvg.String())
}

func RenderTerm(term *midterm.Terminal, depth int) templ.Component {
	var buf bytes.Buffer
	buf.WriteString(`<pre class="terminal">`)

	// Iterate each row. When the css changes, close the previous span, and open
	// a new one. No need to close a span when the css is empty, we won't have
	// opened one in the past.
	var lastFormat midterm.Format
	for y, row := range term.Content {
		var lastIdx int
		for i := len(row) - 1; i >= 0; i-- {
			if row[i] != ' ' || term.Format[y][i] != (midterm.Format{}) {
				lastIdx = i + 1
				break
			}
		}
		for x, r := range row[:lastIdx] {
			f := term.Format[y][x]
			if f != lastFormat {
				if lastFormat != (midterm.Format{}) {
					buf.WriteString("</span>")
				}
				if f != (midterm.Format{}) {
					buf.WriteString(`<span ` + formatAttrs(f) + `>`)
				}
				lastFormat = f
			}
			if s := maybeEscapeRune(r); s != "" {
				buf.WriteString(s)
			} else {
				buf.WriteRune(r)
			}
		}
		buf.WriteRune('\n')
	}
	buf.WriteString("</pre>")

	return templ.Raw(buf.String())
}

func formatAttrs(f midterm.Format) string {
	attrs := []string{}

	classes := []string{}
	styles := []string{}

	if f.Fg != nil {
		fgColor := termenvColorClass(f.Fg)
		if fgColor.class != "" {
			classes = append(classes, "ansi-fg-"+fgColor.class)
		} else {
			attrs = append(attrs, fmt.Sprintf("color: %s;", fgColor.rgb.Hex()))
		}
	}

	if f.Bg != nil {
		fgColor := termenvColorClass(f.Fg)
		if fgColor.class != "" {
			classes = append(classes, "ansi-bg-"+fgColor.class)
		} else {
			attrs = append(attrs, fmt.Sprintf("background: %s;", fgColor.rgb.Hex()))
		}
	}

	if len(classes) > 0 {
		attrs = append(attrs, fmt.Sprintf("class=\"%s\"", strings.Join(classes, " ")))
	}

	if len(styles) > 0 {
		attrs = append(attrs, fmt.Sprintf("style=\"%s\"", strings.Join(styles, " ")))
	}

	return strings.Join(attrs, " ")
}

type cssColor struct {
	class string
	rgb   colorful.Color
}

func termenvColorClass(c termenv.Color) cssColor {
	color := cssColor{}
	switch c {
	case termenv.ANSIBlack:
		color.class = "black"
	case termenv.ANSIRed:
		color.class = "red"
	case termenv.ANSIGreen:
		color.class = "green"
	case termenv.ANSIYellow:
		color.class = "yellow"
	case termenv.ANSIBlue:
		color.class = "blue"
	case termenv.ANSIMagenta:
		color.class = "magenta"
	case termenv.ANSICyan:
		color.class = "cyan"
	case termenv.ANSIWhite:
		color.class = "white"
	case termenv.ANSIBrightBlack:
		color.class = "bright-black"
	case termenv.ANSIBrightRed:
		color.class = "bright-red"
	case termenv.ANSIBrightGreen:
		color.class = "bright-green"
	case termenv.ANSIBrightYellow:
		color.class = "bright-yellow"
	case termenv.ANSIBrightBlue:
		color.class = "bright-blue"
	case termenv.ANSIBrightMagenta:
		color.class = "bright-magenta"
	case termenv.ANSIBrightCyan:
		color.class = "bright-cyan"
	case termenv.ANSIBrightWhite:
		color.class = "bright-white"
	default:
		color.rgb = termenv.ConvertToRGB(c)
	}
	return color
}

// maybeEscapeRune potentially escapes a rune for display in an html document.
// It only escapes the things that html.EscapeString does, but it works without allocating
// a string to hold r. Returns an empty string if there is no need to escape.
func maybeEscapeRune(r rune) string {
	switch r {
	case '&':
		return "&amp;"
	case '\'':
		return "&#39;"
	case '<':
		return "&lt;"
	case '>':
		return "&gt;"
	case '"':
		return "&quot;"
	}
	return ""
}
