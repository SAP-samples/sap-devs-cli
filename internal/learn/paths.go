package learn

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/discovery"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func LoadPaths(packs []*content.Pack) []LearningPath {
	var out []LearningPath
	for _, p := range packs {
		for _, def := range p.LearningPaths {
			path := LearningPath{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				Level:       NormalizeLevel(def.Level),
				PackID:      def.PackID,
				Generated:   false,
			}
			for _, s := range def.Steps {
				path.Steps = append(path.Steps, PathStep{
					Type: ItemType(s.Type),
					Slug: s.Slug,
				})
			}
			out = append(out, path)
		}
	}
	return out
}

func AutoFillPaths(
	packs []*content.Pack,
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
) []LearningPath {
	journeyBySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		journeyBySlug[j.Slug] = j
	}
	tutBySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		tutBySlug[t.Slug] = t
	}
	missionByID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		missionByID[m.ID] = m
	}

	var out []LearningPath
	for _, p := range packs {
		if len(p.LearningPaths) > 0 {
			continue
		}

		// Collect featured items grouped by level
		byLevel := make(map[string][]PathStep)

		for _, ref := range p.LearningRefs {
			if !ref.Featured {
				continue
			}
			if j, ok := journeyBySlug[ref.Slug]; ok {
				lvl := NormalizeLevel(j.Level)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemJourney, Slug: ref.Slug})
			}
		}
		for _, ref := range p.TutorialRefs {
			if !ref.Featured {
				continue
			}
			if t, ok := tutBySlug[ref.Slug]; ok {
				lvl := NormalizeLevel(t.Level)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemTutorial, Slug: ref.Slug})
			}
		}
		for _, ref := range p.DiscoveryMissions {
			if !ref.Featured {
				continue
			}
			if m, ok := missionByID[ref.ID]; ok {
				lvl := effortToLevel(m.Effort)
				byLevel[lvl] = append(byLevel[lvl], PathStep{Type: ItemMission, Slug: fmt.Sprintf("%d", ref.ID)})
			}
		}

		for _, level := range []string{"beginner", "intermediate", "advanced"} {
			steps := byLevel[level]
			if len(steps) < 2 {
				continue
			}
			out = append(out, LearningPath{
				ID:        fmt.Sprintf("%s-%s-auto", p.ID, level),
				Name:      fmt.Sprintf("%s — %s", p.Name, titleCase(level)),
				Level:     level,
				PackID:    p.ID,
				Steps:     steps,
				Generated: true,
			})
		}
	}
	return out
}

func ResolvePaths(
	paths []LearningPath,
	journeys []learning.LearningJourney,
	tuts []tutorials.TutorialMeta,
	missions []discovery.Mission,
) []LearningPath {
	journeyBySlug := make(map[string]learning.LearningJourney, len(journeys))
	for _, j := range journeys {
		journeyBySlug[j.Slug] = j
	}
	tutBySlug := make(map[string]tutorials.TutorialMeta, len(tuts))
	for _, t := range tuts {
		tutBySlug[t.Slug] = t
	}
	missionByID := make(map[int]discovery.Mission, len(missions))
	for _, m := range missions {
		missionByID[m.ID] = m
	}

	resolved := make([]LearningPath, len(paths))
	for i, p := range paths {
		resolved[i] = p
		resolved[i].Steps = make([]PathStep, len(p.Steps))
		for j, step := range p.Steps {
			resolved[i].Steps[j] = step
			switch step.Type {
			case ItemJourney:
				if jr, ok := journeyBySlug[step.Slug]; ok {
					item := journeyToItem(jr, false, p.PackID)
					resolved[i].Steps[j].Item = &item
				}
			case ItemTutorial:
				if t, ok := tutBySlug[step.Slug]; ok {
					item := tutorialToItem(t, false, p.PackID)
					resolved[i].Steps[j].Item = &item
				}
			case ItemMission:
				id, err := strconv.Atoi(step.Slug)
				if err == nil {
					if m, ok := missionByID[id]; ok {
						item := missionToItem(m, false, p.PackID)
						resolved[i].Steps[j].Item = &item
					}
				}
			}
		}
	}
	return resolved
}

func FindPath(paths []LearningPath, id string) *LearningPath {
	for _, p := range paths {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
