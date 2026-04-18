package geo

import "math"

const earthRadiusKm = 6371.0

// DistanceKm returns the great-circle distance in kilometres between two points.
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// IsNearby reports whether two points are within radiusKm of each other.
func IsNearby(lat1, lon1, lat2, lon2 float64, radiusKm float64) bool {
	return DistanceKm(lat1, lon1, lat2, lon2) <= radiusKm
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
