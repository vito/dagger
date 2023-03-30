package tui

import "golang.org/x/exp/constraints"

func max[T constraints.Ordered](i, j T) T {
	if i > j {
		return i
	}
	return j
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func trunc(str string, size int) string {
	if len(str) <= size {
		return str
	}

	return str[:size-1] + "…"
}
