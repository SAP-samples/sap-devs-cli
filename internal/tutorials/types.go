package tutorials

import "time"

// TutorialMeta is a resolved tutorial in the full index (cached from GitHub).
type TutorialMeta struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Time        int      `json:"time"`
	Level       string   `json:"level"`
	Tags        []string `json:"tags"`
	PrimaryTag  string   `json:"primary_tag"`
	Author      string   `json:"author,omitempty"`
	Repo        string   `json:"repo"`
	URL         string   `json:"url"`
	Parser      string   `json:"parser"`
}

// Tutorial is a fully parsed tutorial with step content.
type Tutorial struct {
	TutorialMeta
	Prerequisites string         `json:"prerequisites,omitempty"`
	YouWillLearn  []string       `json:"you_will_learn,omitempty"`
	Steps         []TutorialStep `json:"steps"`
}

// TutorialStep is a single step within a tutorial.
type TutorialStep struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TutorialProgress tracks a user's position within a tutorial.
type TutorialProgress struct {
	Slug           string     `json:"slug"`
	CurrentStep    int        `json:"current_step"`
	CompletedSteps []int      `json:"completed_steps"`
	TotalSteps     int        `json:"total_steps"`
	StartedAt      time.Time  `json:"started_at"`
	LastAccessed   time.Time  `json:"last_accessed"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// RepoInfo stores cached metadata about a sap-tutorials repo.
type RepoInfo struct {
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	TreeSHA       string `json:"tree_sha,omitempty"`
}
