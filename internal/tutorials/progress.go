package tutorials

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const progressFile = "tutorial-progress.json"

func progressPath(dataDir string) string {
	return filepath.Join(dataDir, progressFile)
}

// LoadProgress reads all tutorial progress from the data directory.
func LoadProgress(dataDir string) (map[string]TutorialProgress, error) {
	data, err := os.ReadFile(progressPath(dataDir))
	if os.IsNotExist(err) {
		return make(map[string]TutorialProgress), nil
	}
	if err != nil {
		return nil, err
	}
	var progress map[string]TutorialProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, err
	}
	return progress, nil
}

// SaveProgress writes all tutorial progress to the data directory.
func SaveProgress(dataDir string, progress map[string]TutorialProgress) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(progressPath(dataDir), data, 0644)
}

// GetProgress returns progress for a single tutorial, or nil if not started.
func GetProgress(dataDir, slug string) (*TutorialProgress, error) {
	all, err := LoadProgress(dataDir)
	if err != nil {
		return nil, err
	}
	if p, ok := all[slug]; ok {
		return &p, nil
	}
	return nil, nil
}

// UpdateProgress updates progress for a tutorial, creating a new entry if needed.
func UpdateProgress(dataDir, slug string, currentStep, totalSteps int, markDone bool) error {
	all, err := LoadProgress(dataDir)
	if err != nil {
		return err
	}

	now := time.Now()
	p, exists := all[slug]
	if !exists {
		p = TutorialProgress{
			Slug:       slug,
			TotalSteps: totalSteps,
			StartedAt:  now,
		}
	}

	p.CurrentStep = currentStep
	p.LastAccessed = now
	p.TotalSteps = totalSteps

	if markDone {
		found := false
		for _, s := range p.CompletedSteps {
			if s == currentStep {
				found = true
				break
			}
		}
		if !found {
			p.CompletedSteps = append(p.CompletedSteps, currentStep)
		}
	}

	if len(p.CompletedSteps) >= totalSteps && p.CompletedAt == nil {
		p.CompletedAt = &now
	}

	all[slug] = p
	return SaveProgress(dataDir, all)
}

// MergeCompletedSteps merges a batch of completed step indices into the stored progress for a tutorial.
// Steps are deduplicated and sorted. currentStep is set explicitly if > 0, otherwise inferred from
// the last completed step. If all steps are completed, CompletedAt is stamped.
func MergeCompletedSteps(dataDir, slug string, completedSteps []int, currentStep, totalSteps int) (*TutorialProgress, error) {
	for _, s := range completedSteps {
		if s < 1 || s > totalSteps {
			return nil, fmt.Errorf("step %d out of range 1..%d", s, totalSteps)
		}
	}

	all, err := LoadProgress(dataDir)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	p, exists := all[slug]
	if !exists {
		p = TutorialProgress{
			Slug:       slug,
			TotalSteps: totalSteps,
			StartedAt:  now,
		}
	}

	existing := make(map[int]bool, len(p.CompletedSteps))
	for _, s := range p.CompletedSteps {
		existing[s] = true
	}
	for _, s := range completedSteps {
		if !existing[s] {
			p.CompletedSteps = append(p.CompletedSteps, s)
			existing[s] = true
		}
	}

	sort.Ints(p.CompletedSteps)

	if currentStep > 0 {
		p.CurrentStep = currentStep
	} else if len(p.CompletedSteps) > 0 {
		p.CurrentStep = p.CompletedSteps[len(p.CompletedSteps)-1] + 1
	}

	p.TotalSteps = totalSteps
	p.LastAccessed = now

	if len(p.CompletedSteps) >= totalSteps && p.CompletedAt == nil {
		p.CompletedAt = &now
	}

	all[slug] = p
	if err := SaveProgress(dataDir, all); err != nil {
		return nil, err
	}
	return &p, nil
}
