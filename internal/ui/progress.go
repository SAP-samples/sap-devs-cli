package ui

// MarkerDoneMsg is sent by fetch goroutines when a marker fetch completes.
type MarkerDoneMsg struct {
	PackID string
	Index  int
	Label  string
	Lines  int
	Err    error
}

// MarkerItem represents a single marker sub-item in the sync progress display.
type MarkerItem struct {
	PackID string
	Index  int
	Label  string
	State  string // "fetching", "done", "failed"
	Lines  int
}
