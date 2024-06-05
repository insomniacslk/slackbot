package gmaps

import (
	"context"
	"errors"

	"github.com/insomniacslk/slackbot/pkg/location"
	"googlemaps.github.io/maps"
)

// Search looks for the given location via the GMaps API.
func Search(apiKey string, locName string) (*location.Location, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	r := maps.GeocodingRequest{
		Address: locName,
	}
	resp, err := client.Geocode(context.Background(), &r)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, errors.New("location not found")
	}
	loc := location.Location{
		Name: resp[0].AddressComponents[0].LongName,
		Lat:  resp[0].Geometry.Location.Lat,
		Lng:  resp[0].Geometry.Location.Lng,
	}
	return &loc, nil
}
