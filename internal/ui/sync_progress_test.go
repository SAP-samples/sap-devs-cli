package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncModel_View_AllPending(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseMarkers, PhaseEvents}
	m := newSyncModel(phases)
	view := m.View()
	assert.Contains(t, view, "Syncing SAP developer content")
	assert.Contains(t, view, "content")
	assert.Contains(t, view, "markers")
	assert.Contains(t, view, "events")
	assert.Contains(t, view, "0%")
}

func TestSyncModel_Update_PhaseStart(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseContent})
	assert.Equal(t, statusActive, m.phases[0].status)
}

func TestSyncModel_Update_PhaseDone(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseContent})
	m.Update(PhaseDoneMsg{ID: PhaseContent, Summary: "fetched archive"})
	assert.Equal(t, statusDone, m.phases[0].status)
	assert.Equal(t, "fetched archive", m.phases[0].summary)
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_Update_PhaseSkip(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents}
	m := newSyncModel(phases)
	m.Update(PhaseSkipMsg{ID: PhaseEvents})
	assert.Equal(t, statusSkipped, m.phases[1].status)
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_Update_PhaseFailed(t *testing.T) {
	phases := []PhaseID{PhaseDiscovery}
	m := newSyncModel(phases)
	m.Update(PhaseStartMsg{ID: PhaseDiscovery})
	m.Update(PhaseDoneMsg{ID: PhaseDiscovery, Err: fmt.Errorf("connection refused")})
	assert.Equal(t, statusFailed, m.phases[0].status)
	assert.Contains(t, m.phases[0].summary, "connection refused")
	assert.Equal(t, 1, m.done)
}

func TestSyncModel_ProgressBar_Percent(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseEvents, PhaseDiscovery}
	m := newSyncModel(phases)
	m.Update(PhaseDoneMsg{ID: PhaseContent, Summary: "done"})
	view := m.View()
	assert.Contains(t, view, "33%")
}

func TestSyncModel_Update_SyncDone_Quits(t *testing.T) {
	phases := []PhaseID{PhaseContent}
	m := newSyncModel(phases)
	_, cmd := m.Update(SyncDoneMsg{})
	// tea.Quit returns a function; check it is non-nil
	assert.NotNil(t, cmd)
}

func TestSyncModel_View_MarkerSubItems(t *testing.T) {
	phases := []PhaseID{PhaseMarkers}
	m := newSyncModel(phases)
	m.markers = []MarkerItem{
		{PackID: "cap", Index: 0, Label: "CAP release notes", State: "done", Lines: 42},
		{PackID: "btp", Index: 1, Label: "BTP updates", State: "fetching"},
	}
	view := m.View()
	assert.Contains(t, view, "cap")
	assert.Contains(t, view, "42 lines")
	assert.Contains(t, view, "fetching...")
}

func TestSyncModel_View_ContainsExpectedPhaseLabels(t *testing.T) {
	phases := []PhaseID{PhaseContent, PhaseCompany, PhaseMarkers, PhaseChangelog, PhaseEvents, PhaseYouTube, PhaseDiscovery, PhaseTutorials, PhaseLearning}
	m := newSyncModel(phases)
	view := m.View()
	for _, label := range []string{"content", "company", "markers", "changelog", "events", "youtube", "discovery", "tutorials", "learning"} {
		assert.True(t, strings.Contains(view, label), "expected view to contain %q", label)
	}
}
