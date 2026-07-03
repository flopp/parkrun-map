package utils

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseKML parses a KML file and extracts the tracks and named points.
// It returns a list of tracks (each track is a list of coordinates) and a map of named points to their coordinates.
func ParseKML(data []byte) ([][]Coordinates, map[string]Coordinates, error) {
	tracks := make([][]Coordinates, 0)
	points := make(map[string]Coordinates)

	dec := xml.NewDecoder(bytes.NewReader(data))

	type placemark struct {
		name        string
		point       string
		trackCoords []string
	}

	var pm placemark
	inPlacemark := false
	inPoint := false
	inLineString := false
	inLinearRing := false
	readName := false
	readCoordinates := false
	textBuf := strings.Builder{}

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("parse kml xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Placemark":
				inPlacemark = true
				pm = placemark{}
			case "Point":
				if inPlacemark {
					inPoint = true
				}
			case "LineString":
				if inPlacemark {
					inLineString = true
				}
			case "LinearRing":
				if inPlacemark {
					inLinearRing = true
				}
			case "name":
				if inPlacemark {
					readName = true
					textBuf.Reset()
				}
			case "coordinates":
				if inPlacemark && (inPoint || inLineString || inLinearRing) {
					readCoordinates = true
					textBuf.Reset()
				} else {
					// make sure there are no unexpected coordinates outside of known elements
					return nil, nil, fmt.Errorf("unexpected <coordinates> outside of Point/LineString/LinearRing")
				}
			}

		case xml.CharData:
			if readName || readCoordinates {
				textBuf.Write([]byte(t))
			}

		case xml.EndElement:
			switch t.Name.Local {
			case "name":
				if readName {
					pm.name = strings.TrimSpace(textBuf.String())
					readName = false
				}
			case "coordinates":
				if readCoordinates {
					coordText := strings.TrimSpace(textBuf.String())
					if inPoint {
						pm.point = coordText
					}
					if inLineString || inLinearRing {
						pm.trackCoords = append(pm.trackCoords, coordText)
					}
					readCoordinates = false
				}
			case "Point":
				inPoint = false
			case "LineString":
				inLineString = false
			case "LinearRing":
				inLinearRing = false
			case "Placemark":
				for _, coordText := range pm.trackCoords {
					track, err := parseCoordinateList(coordText)
					if err != nil {
						return nil, nil, fmt.Errorf("parse placemark track '%s': %w", pm.name, err)
					}
					if len(track) > 1 {
						tracks = append(tracks, track)
					}
				}

				if pm.name != "" && pm.point != "" {
					point, err := parseSingleCoordinate(pm.point)
					if err != nil {
						return nil, nil, fmt.Errorf("parse placemark point '%s': %w", pm.name, err)
					}
					points[pm.name] = point
				}

				inPlacemark = false
				inPoint = false
				inLineString = false
				inLinearRing = false
				readName = false
				readCoordinates = false
				textBuf.Reset()
			}
		}
	}

	return tracks, points, nil
}

func parseCoordinateList(value string) ([]Coordinates, error) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty coordinates")
	}

	track := make([]Coordinates, 0, len(parts))
	for _, part := range parts {
		ll, err := parseCoordinateTuple(part)
		if err != nil {
			return nil, err
		}
		track = append(track, ll)
	}

	return track, nil
}

func parseSingleCoordinate(value string) (Coordinates, error) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return Coordinates{}, fmt.Errorf("empty coordinates")
	}

	return parseCoordinateTuple(parts[0])
}

func parseCoordinateTuple(value string) (Coordinates, error) {
	parts := strings.Split(value, ",")
	if len(parts) < 2 {
		return Coordinates{}, fmt.Errorf("invalid coordinate '%s'", value)
	}

	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return Coordinates{}, fmt.Errorf("invalid lon in '%s': %w", value, err)
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return Coordinates{}, fmt.Errorf("invalid lat in '%s': %w", value, err)
	}

	return Coordinates{Lat: lat, Lon: lon}, nil
}
