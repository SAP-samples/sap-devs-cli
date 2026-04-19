package learning

import "time"

const (
	CacheTTL       = 7 * 24 * time.Hour // 7 days
	SearchCacheTTL = 1 * time.Hour

	CatalogURL = "https://learning.sap.com/service/catalog-download/json"
	SearchURL  = "https://learning.sap.com/service/learning/search/getCards"
	BaseURL    = "https://learning.sap.com/learning-journeys/"
)

// LearningJourney is the cached index entry for a single learning journey.
type LearningJourney struct {
	ObjectID        string   `json:"objectId"`
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	Level           string   `json:"level"`
	DurationHours   string   `json:"durationHours"`
	Roles           []string `json:"roles"`
	Product         string   `json:"product"`
	ProductCategory string   `json:"productCategory"`
	ProductSubcat   string   `json:"productSubcat"`
	Objectives      string   `json:"objectives"`
	AvailableFrom   string   `json:"availableFrom"`
	URL             string   `json:"url"`
}

// catalogItem is the raw JSON shape from the catalog download endpoint.
type catalogItem struct {
	LearningType     string     `json:"Learning_type"`
	LearningObjectID string     `json:"Learning_object_ID"`
	Title            string     `json:"Title"`
	Description      string     `json:"Description"`
	Level            string     `json:"Level"`
	DurationInHours  string     `json:"Duration_in_hours"`
	Role             string     `json:"Role"`
	Product          string     `json:"LSC_product"`
	ProductCategory  string     `json:"LSC_product_category"`
	ProductSubcat    string     `json:"LSC_product_subcategory"`
	Objectives       string     `json:"Learning_objectives"`
	AvailableFrom    string     `json:"Content_available_from"`
	DirectLink       directLink `json:"Direct_link"`
}

type directLink struct {
	Hyperlink string `json:"hyperlink"`
}

// searchResponse is the envelope from the getCards search API.
type searchResponse struct {
	Value searchValue `json:"value"`
}

type searchValue struct {
	Results    []searchResult `json:"results"`
	TotalCount int            `json:"totalCount"`
	NextPage   *int           `json:"nextPage"`
}

type searchResult struct {
	Title           string   `json:"title"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	ExperienceLevel string   `json:"experienceLevel"`
	Duration        float64  `json:"duration"`
	Roles           []string `json:"roles"`
	ObjType         string   `json:"objType"`
}
