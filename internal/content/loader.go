package content

import (
	"os"
	"path/filepath"
	"sort"
)

// ContentLoader merges packs from multiple layers: official → company → user → project.
type ContentLoader struct {
	OfficialDir string
	CompanyDir  string // empty if not configured
	UserDir     string
	ProjectDir  string // empty if not in a project
}

// LoadPacks loads and merges packs from all configured layers,
// then orders them by the given profile. Later layers override earlier ones by pack ID.
func (cl *ContentLoader) LoadPacks(profile *Profile, lang string) ([]*Pack, error) {
	packMap := make(map[string]*Pack)

	for _, dir := range cl.activeDirs() {
		packsDir := filepath.Join(dir, "packs")
		entries, err := os.ReadDir(packsDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pack, err := LoadPack(filepath.Join(packsDir, e.Name()), lang)
			if err != nil {
				return nil, err
			}
			packMap[pack.ID] = pack // later layers override
		}
	}

	packs := make([]*Pack, 0, len(packMap))
	for _, p := range packMap {
		packs = append(packs, p)
	}
	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Weight > packs[j].Weight
	})
	weighted := ApplyWeights(packs, profile)

	// Pin base packs first. Base packs are exempt from profile weight ordering —
	// they always appear before non-base packs regardless of their weight value.
	// Among multiple base packs, relative order is preserved from the weight sort above.
	var base, nonBase []*Pack
	for _, p := range weighted {
		if p.Base {
			base = append(base, p)
		} else {
			nonBase = append(nonBase, p)
		}
	}
	return append(base, nonBase...), nil
}

// LoadProfiles loads profiles from all configured layers (later layers override).
func (cl *ContentLoader) LoadProfiles() ([]*Profile, error) {
	profileMap := make(map[string]*Profile)
	for _, dir := range cl.activeDirs() {
		profilesDir := filepath.Join(dir, "profiles")
		profiles, err := LoadProfiles(profilesDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			profileMap[p.ID] = p
		}
	}
	result := make([]*Profile, 0, len(profileMap))
	for _, p := range profileMap {
		result = append(result, p)
	}
	return result, nil
}

// FindProfile returns a profile by ID from all layers, or nil if not found.
func (cl *ContentLoader) FindProfile(id string) (*Profile, error) {
	profiles, err := cl.LoadProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func (cl *ContentLoader) activeDirs() []string {
	dirs := []string{cl.OfficialDir}
	if cl.CompanyDir != "" {
		dirs = append(dirs, cl.CompanyDir)
	}
	if cl.UserDir != "" {
		dirs = append(dirs, cl.UserDir)
	}
	if cl.ProjectDir != "" {
		dirs = append(dirs, cl.ProjectDir)
	}
	return dirs
}
