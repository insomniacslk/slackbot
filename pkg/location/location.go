package location

import "fmt"

// Location holds a location name, latitude and longitude.
type Location struct {
	Name     string
	Lat, Lng float64
}

// LatString returns latitude as a string.
func (l *Location) LatString() string {
	return fmt.Sprintf("%f", l.Lat)
}

// LngString returns longitude as a string.
func (l *Location) LngString() string {
	return fmt.Sprintf("%f", l.Lng)
}
