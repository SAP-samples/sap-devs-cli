package learn

import (
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestSearch_TitleMatchFirst(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Getting Started with CAP", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "Advanced Integration", Level: "ADVANCED", Description: "Uses CAP framework", Product: "SAP BTP"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "CAP Tutorial Basics", Level: "beginner", Time: 30},
	}
	missions := []discovery.Mission{
		{ID: 1, Name: "Build with CAP", Effort: "1", Product: "SAP BTP"},
	}

	results := Search(journeys, tuts, missions, "CAP", RecommendOptions{Limit: 10, All: true})

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Title matches should come before description-only matches
	for _, r := range results[:3] {
		if r.Slug == "j2" {
			t.Errorf("description-only match (j2) should not appear before title matches")
		}
	}
}

func TestSearch_LevelFilter(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "CAP Beginner", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "CAP Advanced", Level: "ADVANCED", Product: "SAP BTP"},
	}

	results := Search(journeys, nil, nil, "CAP", RecommendOptions{Level: "beginner", Limit: 10, All: true})

	if len(results) != 1 {
		t.Fatalf("expected 1 result after level filter, got %d", len(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "HANA Cloud Setup", Level: "beginner", Time: 20},
	}

	results := Search(nil, tuts, nil, "hana", RecommendOptions{Limit: 10, All: true})

	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive search, got %d", len(results))
	}
}

func TestSearch_Limit(t *testing.T) {
	journeys := make([]learning.LearningJourney, 20)
	for i := range journeys {
		journeys[i] = learning.LearningJourney{Slug: "j", Title: "Match", Level: "BEGINNER", Product: "X"}
	}

	results := Search(journeys, nil, nil, "Match", RecommendOptions{Limit: 3, All: true})

	if len(results) != 3 {
		t.Fatalf("expected 3 results after limit, got %d", len(results))
	}
}
