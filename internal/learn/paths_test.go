package learn

import (
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/learning"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestLoadPaths(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			LearningPaths: []content.LearningPathDef{
				{
					ID:    "cap-start",
					Name:  "Getting Started",
					Level: "beginner",
					Steps: []content.LearningPathStepDef{
						{Type: "journey", Slug: "j1"},
						{Type: "tutorial", Slug: "t1"},
					},
					PackID: "cap",
				},
			},
		},
	}

	paths := LoadPaths(packs)

	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].ID != "cap-start" {
		t.Errorf("expected id cap-start, got %s", paths[0].ID)
	}
	if paths[0].Generated {
		t.Errorf("expected curated path, got generated")
	}
	if len(paths[0].Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(paths[0].Steps))
	}
}

func TestAutoFillPaths_MinTwoItems(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			TutorialRefs: []content.TutorialRef{
				{Slug: "t1", Featured: true, PackID: "cap"},
				{Slug: "t2", Featured: true, PackID: "cap"},
			},
		},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tut 1", Level: "beginner", Time: 30},
		{Slug: "t2", Title: "Tut 2", Level: "beginner", Time: 45},
	}

	paths := AutoFillPaths(packs, nil, tuts, nil)

	if len(paths) != 1 {
		t.Fatalf("expected 1 auto-filled path, got %d", len(paths))
	}
	if !paths[0].Generated {
		t.Errorf("expected generated flag to be true")
	}
	if paths[0].Level != "beginner" {
		t.Errorf("expected level beginner, got %s", paths[0].Level)
	}
}

func TestAutoFillPaths_SkipsPackWithCuratedPaths(t *testing.T) {
	packs := []*content.Pack{
		{
			ID:   "cap",
			Name: "SAP CAP",
			LearningPaths: []content.LearningPathDef{
				{ID: "existing", Name: "Existing", PackID: "cap"},
			},
			TutorialRefs: []content.TutorialRef{
				{Slug: "t1", Featured: true, PackID: "cap"},
				{Slug: "t2", Featured: true, PackID: "cap"},
			},
		},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tut 1", Level: "beginner", Time: 30},
		{Slug: "t2", Title: "Tut 2", Level: "beginner", Time: 45},
	}

	paths := AutoFillPaths(packs, nil, tuts, nil)

	if len(paths) != 0 {
		t.Fatalf("expected 0 auto-filled paths (pack has curated), got %d", len(paths))
	}
}

func TestResolvePaths_PopulatesItems(t *testing.T) {
	paths := []LearningPath{
		{
			ID: "test",
			Steps: []PathStep{
				{Type: ItemJourney, Slug: "j1"},
				{Type: ItemTutorial, Slug: "t1"},
				{Type: ItemMission, Slug: "999"},
			},
		},
	}
	journeys := []learning.LearningJourney{
		{Slug: "j1", Title: "Journey", Level: "BEGINNER", DurationHours: "4"},
	}
	tuts := []tutorials.TutorialMeta{
		{Slug: "t1", Title: "Tutorial", Level: "beginner", Time: 30},
	}

	resolved := ResolvePaths(paths, journeys, tuts, nil)

	if resolved[0].Steps[0].Item == nil {
		t.Fatal("expected journey step to be resolved")
	}
	if resolved[0].Steps[1].Item == nil {
		t.Fatal("expected tutorial step to be resolved")
	}
	if resolved[0].Steps[2].Item != nil {
		t.Fatal("expected unmatched mission step to be nil")
	}
}
