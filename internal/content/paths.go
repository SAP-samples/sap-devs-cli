package content

// LearningPathDef is the YAML-parsed shape from paths.yaml.
type LearningPathDef struct {
	ID          string                `yaml:"id"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Level       string                `yaml:"level,omitempty"`
	Steps       []LearningPathStepDef `yaml:"steps"`
	PackID      string                // set at load time
}

// LearningPathStepDef is a single step in a curated path.
type LearningPathStepDef struct {
	Type string `yaml:"type"`
	Slug string `yaml:"slug"`
}

// FlattenLearningPaths collects all curated learning paths from all packs.
func FlattenLearningPaths(packs []*Pack) []LearningPathDef {
	var out []LearningPathDef
	for _, p := range packs {
		out = append(out, p.LearningPaths...)
	}
	return out
}
