package discovery

// Mission represents a single mission from GetMissionCatalogContentV2 or search.
type Mission struct {
	ID                 int    `json:"Id"`
	Name               string `json:"Name"`
	Category           string `json:"Category"`
	SubCategory        string `json:"SubCategory"`
	Product            string `json:"Product"`
	Industry           string `json:"Industry"`
	LoB                string `json:"LoB"`
	FocusTags          string `json:"FocusTags"`
	Type               string `json:"Type"`
	PartnerCompany     string `json:"PartnerCompany"`
	ReferenceCustomers string `json:"ReferenceCustomers"`
	UCId               int    `json:"UCId"`
	UCLongDescription  string `json:"UCLongDescription"`
	UCRibbonText       string `json:"UCRibbonText"`
	Effort             string `json:"Effort"`
	MissionCount       int    `json:"MissionCount"`
}

// MissionCatalogGroup is a named group returned by GetMissionCatalogContentV2.
type MissionCatalogGroup struct {
	Name     string    `json:"name"`
	Desc     string    `json:"desc"`
	Missions []Mission `json:"missions"`
}

// Service represents a BTP service from /servicecatalog/ServiceDetailss.
type Service struct {
	ID                  string `json:"Id"`
	Name                string `json:"Name"`
	ShortName           string `json:"ShortName"`
	Category            string `json:"Category"`
	ShortDescription    string `json:"ShortDescription"`
	LicenseModelType    string `json:"LicenseModelType"`
	IsDeprecatedService bool   `json:"IsDeprecatedService"`
}

// GuidanceNode is one node in the guidance framework tree.
type GuidanceNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Domain   *string        `json:"domain"`
	Order    int            `json:"order"`
	Children []GuidanceNode `json:"children"`
}

// ProductCategory is one entry in the products/categories taxonomy.
type ProductCategory struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Products []ProductCategory `json:"products,omitempty"`
}

// Categories wraps the GetProductsCategories response.
type Categories struct {
	Products []ProductCategory `json:"products"`
}

// Facets wraps the GetApplicationFocusTagsIndustryLob response.
type Facets struct {
	FocusTags  []FacetItem `json:"focusTags"`
	Industries []FacetItem `json:"industries"`
	Lobs       []FacetItem `json:"lobs"`
}

// FacetItem is a single tag/industry/LOB entry.
type FacetItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SearchFilters controls server-side filtering for GetViewFuzzySearchesCustomV3.
type SearchFilters struct {
	Category  string
	Product   string
	LoB       string
	Industry  string
	FocusTags string
	Partners  string
	Top       int
}
