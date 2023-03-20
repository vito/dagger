package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"golang.org/x/exp/constraints"
)

type TreeEntry interface {
	tea.Model

	ID() string
	Name() string

	Entries() []TreeEntry

	Started() *time.Time
	Completed() *time.Time
	Cached() bool
	Error() string

	SetWidth(int)
	SetHeight(int)
	ScrollPercent() float64
}

type Tree struct {
	viewport viewport.Model

	root    TreeEntry
	current TreeEntry
	focus   bool

	spinner   spinner.Model
	collapsed map[TreeEntry]bool
}

func (m *Tree) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *Tree) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *Tree) SetRoot(root TreeEntry) {
	m.root = root
	if m.current == nil && len(root.Entries()) > 0 {
		m.current = root.Entries()[0]
	}
}

func (m *Tree) SetWidth(width int) {
	m.viewport.Width = width
}

func (m *Tree) SetHeight(height int) {
	m.viewport.Height = height
}

func (m *Tree) UsedHeight() int {
	if m.root == nil {
		return 0
	}

	return m.height(m.root) - 1 // 'root' node isn't shown
}

func (m Tree) Current() TreeEntry {
	return m.current
}

func (m *Tree) Focus(focus bool) {
	m.focus = focus
}

func (m *Tree) View() string {
	if m.root == nil {
		return ""
	}
	offset := m.currentOffset(m.root) - 1
	views := []string{}
	entries := m.root.Entries()
	for i, item := range entries {
		if i == len(entries)-1 {
			views = append(views, m.itemView(item, []bool{true}))
		} else {
			views = append(views, m.itemView(item, []bool{false}))
		}
	}

	m.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, views...))

	if offset >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(offset - m.viewport.Height + 1)
	}

	if offset < m.viewport.YOffset {
		m.viewport.SetYOffset(offset)
	}

	return m.viewport.View()
}

func (m *Tree) treePrefixView(padding []bool) string {
	pad := strings.Builder{}
	for i, last := range padding {
		leaf := i == len(padding)-1

		switch {
		case leaf && !last:
			pad.WriteString("├─")
		case leaf && last:
			pad.WriteString("└─")
		case !leaf && !last:
			pad.WriteString("│ ")
		case !leaf && last:
			pad.WriteString("  ")
		}
	}
	return pad.String()
}

func (m *Tree) statusView(item TreeEntry) string {
	if item.Cached() {
		return cachedStatus.String()
	}
	if item.Error() != "" {
		return failedStatus.String()
	}
	if item.Started() != nil {
		if item.Completed() != nil {
			return completedStatus.String()
		}
		return m.spinner.View()
	}
	return " "
}

func (m *Tree) timerView(item TreeEntry) string {
	if item.Started() == nil {
		return ""
	}
	if item.Cached() {
		return itemTimerStyle.Render("CACHED ")
	}
	done := item.Completed()
	if done == nil {
		now := time.Now()
		done = &now
	}
	diff := done.Sub(*item.Started())

	prec := 1
	sec := diff.Seconds()
	if sec < 10 {
		prec = 2
	} else if sec < 100 {
		prec = 1
	}
	return itemTimerStyle.Render(fmt.Sprintf("%.[2]*[1]fs ", sec, prec))
}

func (m *Tree) currentOffset(item TreeEntry) int {
	if item == m.current {
		return 0
	}

	offset := 1

	entries := item.Entries()
	for i, entry := range entries {
		if entry == item {
			return i
		}

		if !m.collapsed[entry] {
			entryOffset := m.currentOffset(entry)
			if entryOffset != -1 {
				return offset + entryOffset
			}
		}

		offset += m.height(entry)
	}

	return -1
}

func (m *Tree) height(item TreeEntry) int {
	return lipgloss.Height(m.itemView(item, []bool{false}))
}

func (m *Tree) itemView(item TreeEntry, padding []bool) string {
	status := " " + m.statusView(item) + " "
	treePrefix := m.treePrefixView(padding)
	expandView := ""
	if item.Entries() != nil {
		if collapsed := m.collapsed[item]; collapsed {
			expandView = "▶ "
		} else {
			expandView = "▼ "
		}
	}
	timerView := m.timerView(item)

	nameWidth := m.viewport.Width - lipgloss.Width(status) - lipgloss.Width(treePrefix) - lipgloss.Width(timerView)
	nameView := lipgloss.NewStyle().
		Width(max(0, nameWidth)).
		Render(" " + expandView + truncate.StringWithTail(item.Name(), uint(nameWidth)-2, "…"))

	view := status + treePrefix
	if item == m.current {
		if m.focus {
			view += selectedStyle.Render(nameView + timerView)
		} else {
			view += selectedStyleBlur.Render(nameView + timerView)
		}
	} else {
		view += nameView + timerView
	}

	entries := item.Entries()
	if entries == nil || m.collapsed[item] {
		return view
	}

	renderedItems := []string{
		view,
	}
	for i, s := range entries {
		pad := append([]bool{}, padding...)
		if i == len(entries)-1 {
			pad = append(pad, true)
		} else {
			pad = append(pad, false)
		}

		renderedItems = append(renderedItems, m.itemView(s, pad))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		renderedItems...,
	)
}

func max[T constraints.Ordered](i, j T) T {
	if i > j {
		return i
	}
	return j
}

func clamp[T constraints.Ordered](min, max, val T) T {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func (m *Tree) MoveUp() {
	prev := m.findPrev(m.current)
	if prev == nil || prev == m.root {
		entries := m.root.Entries()
		prev = entries[len(entries)-1]
	}
	m.current = prev
}

func (m *Tree) MoveDown() {
	next := m.findNext(m.current)
	if next == nil {
		next = m.root.Entries()[0]
	}
	m.current = next
}

func (m *Tree) Collapse(entry TreeEntry, recursive bool) {
	m.setCollapsed(entry, true, recursive)
}

func (m *Tree) Expand(entry TreeEntry, recursive bool) {
	m.setCollapsed(entry, false, recursive)
}

func (m *Tree) setCollapsed(entry TreeEntry, collapsed, recursive bool) {
	// Non collapsible
	if entry == nil || entry.Entries() == nil {
		return
	}
	m.collapsed[entry] = collapsed
	if !recursive {
		return
	}
	for _, e := range entry.Entries() {
		m.setCollapsed(e, collapsed, recursive)
	}
}

func (m *Tree) Follow() {
	if m.root == nil {
		return
	}

	if m.current == nil {
		return
	}

	if m.current.Completed() == nil && len(m.current.Entries()) == 0 {
		return
	}

	entry := m.current
	for {
		entry = m.findNext(entry)
		if entry == nil {
			return
		}
		if len(entry.Entries()) > 0 {
			continue
		}
		if entry.Started() != nil && entry.Completed() == nil && !entry.Cached() {
			m.current = entry
			return
		}
	}
}

// findParent returns the parent entry containing the given `entry`
func (m *Tree) findParent(group TreeEntry, entry TreeEntry) TreeEntry {
	entries := group.Entries()
	for _, e := range entries {
		if e == entry {
			return group
		}
		if found := m.findParent(e, entry); found != nil {
			return found
		}
	}
	return nil
}

// findSibilingAfter returns the entry immediately after the specified entry within the same parent.
// `nil` if not found or if entry is the last entry.
func (m *Tree) findSibilingAfter(parent, entry TreeEntry) TreeEntry {
	entries := parent.Entries()
	for i, e := range entries {
		if e != entry {
			continue
		}
		newPos := i + 1
		if newPos >= len(entries) {
			return nil
		}
		return entries[newPos]
	}
	return nil
}

// findSibilingBefore returns the entry immediately preceding the specified entry within the same parent.
// `nil` if not found or if entry is the first entry.
func (m *Tree) findSibilingBefore(parent, entry TreeEntry) TreeEntry {
	entries := parent.Entries()
	for i, e := range entries {
		if e != entry {
			continue
		}
		newPos := i - 1
		if newPos < 0 {
			return nil
		}
		return entries[newPos]
	}
	return nil
}

func (m *Tree) findNext(entry TreeEntry) TreeEntry {
	// If this entry has entries, pick the first child
	if entries := entry.Entries(); !m.collapsed[entry] && len(entries) > 0 {
		return entries[0]
	}

	// Otherwise, pick the next sibiling in the same parent group
	parent := m.findParent(m.root, entry)
	for {
		if next := m.findSibilingAfter(parent, entry); next != nil {
			return next
		}
		// We reached the end of the group, try again with the grand-parent
		entry = parent
		parent = m.findParent(m.root, entry)
		if parent == nil {
			return nil
		}
	}
}

func (m *Tree) findPrev(entry TreeEntry) TreeEntry {
	parent := m.findParent(m.root, entry)
	prev := m.findSibilingBefore(parent, entry)
	// If there's no previous element, pick the parent.
	if prev == nil {
		return parent
	}
	// If the previous sibiling is a group, go to the last element recursively
	for {
		entries := prev.Entries()
		if m.collapsed[prev] || len(entries) == 0 {
			return prev
		}
		prev = entries[len(entries)-1]
	}
}
