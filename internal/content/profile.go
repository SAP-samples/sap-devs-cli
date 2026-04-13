package content

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Profile is a developer persona that weights packs by relevance.
type Profile struct {
	ID          string       `yaml:"id"`
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Packs       []PackWeight `yaml:"packs"`
	TipTags     []string     `yaml:"tip_tags"`
}

// PackWeight pairs a pack ID with a priority weight.
type PackWeight struct {
	ID     string `yaml:"id"`
	Weight int    `yaml:"weight"`
}

// LoadProfile reads a profile YAML file.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	return &p, yaml.Unmarshal(data, &p)
}

// LoadProfiles reads all *.yaml files from profilesDir.
func LoadProfiles(profilesDir string) ([]*Profile, error) {
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, err
	}
	var profiles []*Profile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p, err := LoadProfile(filepath.Join(profilesDir, e.Name()))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// ApplyWeights returns packs sorted by the profile's weight declarations.
// Packs not mentioned by the profile retain their base weight.
func ApplyWeights(packs []*Pack, profile *Profile) []*Pack {
	if profile == nil {
		return packs
	}
	weightMap := make(map[string]int)
	for _, pw := range profile.Packs {
		weightMap[pw.ID] = pw.Weight
	}
	result := make([]*Pack, len(packs))
	copy(result, packs)
	sort.SliceStable(result, func(i, j int) bool {
		wi := weightMap[result[i].ID]
		if wi == 0 {
			wi = result[i].Weight
		}
		wj := weightMap[result[j].ID]
		if wj == 0 {
			wj = result[j].Weight
		}
		return wi > wj
	})
	return result
}
