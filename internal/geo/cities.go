package geo

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed cities.json
var citiesJSON []byte

type city struct {
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

var (
	cities     []city
	citiesOnce sync.Once
)

func loadCities() {
	citiesOnce.Do(func() {
		_ = json.Unmarshal(citiesJSON, &cities)
	})
}

// Lookup resolves a location string like "Hamburg, Germany" to coordinates.
// Tries exact city+country match first, then city-only. Case-insensitive.
func Lookup(location string) (lat, lon float64, ok bool) {
	loadCities()
	loc := strings.TrimSpace(location)
	if loc == "" || strings.EqualFold(loc, "virtual") {
		return 0, 0, false
	}

	parts := strings.SplitN(loc, ",", 2)
	cityName := strings.TrimSpace(parts[0])
	countryName := ""
	if len(parts) > 1 {
		countryName = strings.TrimSpace(parts[1])
	}

	// Exact city+country match
	if countryName != "" {
		for _, c := range cities {
			if strings.EqualFold(c.Name, cityName) && strings.EqualFold(c.Country, countryName) {
				return c.Lat, c.Lon, true
			}
		}
	}

	// City-only match
	for _, c := range cities {
		if strings.EqualFold(c.Name, cityName) {
			return c.Lat, c.Lon, true
		}
	}

	return 0, 0, false
}
