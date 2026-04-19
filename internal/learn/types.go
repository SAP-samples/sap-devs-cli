package learn

// ItemType identifies the source content type for a learn item.
type ItemType string

const (
	ItemJourney  ItemType = "journey"
	ItemTutorial ItemType = "tutorial"
	ItemMission  ItemType = "mission"
)

// LearnItem is a unified wrapper around content from any source.
type LearnItem struct {
	Type     ItemType
	Title    string
	Slug     string
	Level    string
	Duration string
	URL      string
	Featured bool
	PackID   string
	Product  string
}

// Recommendations holds profile-filtered items grouped by type.
type Recommendations struct {
	Journeys  []LearnItem
	Tutorials []LearnItem
	Missions  []LearnItem
}

// RecommendOptions controls filtering for recommendations and search.
type RecommendOptions struct {
	Level  string
	PackID string
	All    bool
	Limit  int
}

// LearningPath is a named, ordered sequence of learn items.
type LearningPath struct {
	ID          string
	Name        string
	Description string
	Level       string
	PackID      string
	Steps       []PathStep
	Generated   bool
}

// PathStep is a single step in a learning path.
type PathStep struct {
	Type ItemType
	Slug string
	Item *LearnItem
}
