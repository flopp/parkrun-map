package utils

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

type Coordinates struct {
	Lat, Lon float64
}

var (
	InvalidCoordinates = Coordinates{100, 0}
)

func (c Coordinates) IsValid() bool {
	return c.Lat <= 90
}

func deg2rad(d float64) float64 {
	return d * math.Pi / 180.0
}

func DistanceMeters(aa, bb Coordinates) float64 {
	const earthRadiusKM float64 = 6371.0

	lat1 := deg2rad(aa.Lat)
	lon1 := deg2rad(aa.Lon)
	lat2 := deg2rad(bb.Lat)
	lon2 := deg2rad(bb.Lon)

	dlat := lat2 - lat1
	dlon := lon2 - lon1

	a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	distance := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a)) * earthRadiusKM * 1000

	return distance
}

func ParseCoordinates(str string) (Coordinates, error) {
	if str == "" {
		return InvalidCoordinates, nil
	}
	r := regexp.MustCompile(`^\s*(-?[0-9\.]+)\s+(-?[0-9\.]+)\s*$`)
	if m := r.FindStringSubmatch(str); m != nil {
		lat, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", str)
		}
		lon, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", str)
		}
		return Coordinates{Lat: lat, Lon: lon}, nil
	}
	r = regexp.MustCompile(`^\s*(-?[0-9\.]+),(-?[0-9\.]+)\s*$`)
	if m := r.FindStringSubmatch(str); m != nil {
		lat, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", str)
		}
		lon, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", str)
		}
		return Coordinates{Lat: lat, Lon: lon}, nil
	}
	return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", str)
}
