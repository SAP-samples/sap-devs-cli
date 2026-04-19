package learn

import (
	"fmt"
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/discovery"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestRecommend_FeaturedFirst(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey One", Level: "BEGINNER", DurationHours: "4", Product: "SAP BTP"},
		{Slug: "j2", Title: "Journey Two", Level: "INTERMEDIATE", DurationHours: "8", Product: "SAP BTP"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tutorial One", Level: "beginner", Time: 45},
		{Slug: "t2", Title: "Tutorial Two", Level: "advanced", Time: 60},
	}
	missions := []discovery.Mission{
		{ID: 100, Name: "Mission One", Effort: "1", Product: "SAP BTP"},
	}
	packs := []*content.Pack{
		{
			ID: "cap",
			LearningRefs:    []content.LearningRef{{Slug: "j1", Featured: true, PackID: "cap"}},
			TutorialRefs:    []content.TutorialRef{{Slug: "t1", Featured: true, PackID: "cap"}},
			DiscoveryMissions: []content.DiscoveryMissionRef{{ID: 100, Featured: true, PackID: "cap"}},
			LearningFilters: &content.LearningProfileFilters{Products: []string{"SAP BTP"}},
		},
	}

	recs := Recommend(journeys, tuts, missions, packs, RecommendOptions{Limit: 10, All: true})

	if len(recs.Journeys) != 2 {
		t.Fatalf("expected 2 journeys, got %d", len(recs.Journeys))
	}
	if recs.Journeys[0].Slug != "j1" || !recs.Journeys[0].Featured {
		t.Errorf("expected first journey to be featured j1, got %s (featured=%v)", recs.Journeys[0].Slug, recs.Journeys[0].Featured)
	}
	if recs.Journeys[0].Level != "beginner" {
		t.Errorf("expected normalized level 'beginner', got %q", recs.Journeys[0].Level)
	}

	if len(recs.Tutorials) != 2 {
		t.Fatalf("expected 2 tutorials, got %d", len(recs.Tutorials))
	}
	if !recs.Tutorials[0].Featured {
		t.Errorf("expected first tutorial to be featured")
	}

	if len(recs.Missions) != 1 {
		t.Fatalf("expected 1 mission, got %d", len(recs.Missions))
	}
	if recs.Missions[0].Level != "beginner" {
		t.Errorf("expected mission effort 1 → beginner, got %q", recs.Missions[0].Level)
	}
}

func TestRecommend_LevelFilter(t *testing.T) {
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey One", Level: "BEGINNER", Product: "SAP BTP"},
		{Slug: "j2", Title: "Journey Two", Level: "ADVANCED", Product: "SAP BTP"},
	}
	packs := []*content.Pack{{ID: "cap"}}

	recs := Recommend(journeys, nil, nil, packs, RecommendOptions{Level: "beginner", Limit: 10, All: true})

	if len(recs.Journeys) != 1 {
		t.Fatalf("expected 1 journey after level filter, got %d", len(recs.Journeys))
	}
	if recs.Journeys[0].Slug != "j1" {
		t.Errorf("expected j1, got %s", recs.Journeys[0].Slug)
	}
}

func TestRecommend_Limit(t *testing.T) {
	journeys := make([]learning.LearningJourney, 20)
	for i := range journeys {
		journeys[i] = learning.LearningJourney{Slug: fmt.Sprintf("j%d", i), Title: "J", Level: "BEGINNER", Product: "X"}
	}
	packs := []*content.Pack{{ID: "cap"}}

	recs := Recommend(journeys, nil, nil, packs, RecommendOptions{Limit: 5, All: true})

	if len(recs.Journeys) != 5 {
		t.Fatalf("expected 5 journeys after limit, got %d", len(recs.Journeys))
	}
}
