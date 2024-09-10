package dagui

import (
	"time"
)

type FrontendOpts struct {
	// Debug tells the frontend to show everything and do one big final render.
	Debug bool

	// Silent tells the frontend to not display progress at all.
	Silent bool

	// Verbosity is the level of detail to show in the TUI.
	Verbosity int

	// Don't show things that completed beneath this duration. (default 100ms)
	TooFastThreshold time.Duration

	// Remove completed things after this duration. (default 1s)
	GCThreshold time.Duration

	// Open web browser with the trace URL as soon as pipeline starts.
	OpenWeb bool

	// Leave the TUI running instead of exiting after completion.
	NoExit bool

	// ZoomedSpan configures a span to be zoomed in on, revealing
	// its child spans.
	ZoomedSpan SpanID

	// FocusedSpan is the currently selected span, i.e. the cursor position.
	FocusedSpan SpanID
}

const (
	HideCompletedVerbosity    = 0
	ShowCompletedVerbosity    = 1
	ExpandCompletedVerbosity  = 2
	ShowInternalVerbosity     = 3
	ShowEncapsulatedVerbosity = 3
	ShowSpammyVerbosity       = 4
	ShowDigestsVerbosity      = 4
)

func (opts FrontendOpts) ShouldShow(span *Span) bool {
	if opts.Debug {
		// debug reveals all
		return true
	}
	if opts.FocusedSpan == span.ID {
		// prevent focused span from disappearing
		return true
	}
	if span.Hidden(opts) {
		return false
	}
	if span.IsFailedOrCausedFailure() {
		return true
	}
	if span.IsPending() {
		return true
	}
	if span.IsRunningOrLinksRunning() {
		return true
	}
	// TODO: avoid breaking chains
	// if opts.TooFastThreshold > 0 &&
	// 	span.ActiveDuration(time.Now()) < opts.TooFastThreshold &&
	// 	opts.Verbosity < ShowSpammyVerbosity {
	// 	// ignore fast steps; signal:noise is too poor
	// 	return false
	// }
	// TODO: bring back <100ms?
	if opts.GCThreshold > 0 &&
		time.Since(span.EndTime) > opts.GCThreshold &&
		opts.Verbosity < ShowCompletedVerbosity {
		// stop showing steps that ended after a given threshold
		return false
	}
	// TODO: don't break chains
	// if opts.TooFastThreshold > 0 &&
	// 	span.ActiveDuration(time.Now()) < opts.TooFastThreshold &&
	// 	opts.Verbosity < ShowSpammyVerbosity {
	// 	// ignore fast steps; signal:noise is too poor
	// 	return false
	// }
	return true
}
