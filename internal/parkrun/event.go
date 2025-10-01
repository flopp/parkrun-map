package parkrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/flopp/go-parkrunparser"
	"github.com/flopp/parkrun-map/internal/utils"
	simplifier "github.com/yrsh/simplify-go"
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

type Event struct {
	Id               string
	Name             string
	Location         string
	SpecificLocation string
	Coords           Coordinates
	CountryUrl       string
	GoogleMapsId     string
	RouteType        string
	Tracks           [][]Coordinates
	Status           string
}

func (event Event) Active() bool {
	return event.Status == ""
}
func (event Event) Planned() bool {
	return event.Status == "geplant"
}
func (event Event) Archived() bool {
	return !event.Active() && !event.Planned()
}

func (event Event) Url() string {
	if event.CountryUrl == "" {
		return fmt.Sprintf("https://www.parkrun.com.de/%s", event.Id)
	}
	return fmt.Sprintf("https://%s/%s", event.CountryUrl, event.Id)
}

func (event Event) CoursePageUrl() string {
	if event.CountryUrl == "" {
		return fmt.Sprintf("https://www.parkrun.com.de/%s/course", event.Id)
	}
	return fmt.Sprintf("https://%s/%s/course", event.CountryUrl, event.Id)
}

func (event Event) ResultsUrl() string {
	if event.CountryUrl == "" {
		return fmt.Sprintf("https://www.parkrun.com.de/%s/results/eventhistory", event.Id)
	}
	return fmt.Sprintf("https://%s/%s/results/eventhistory", event.CountryUrl, event.Id)
}

func (event Event) WikiUrl() string {
	return fmt.Sprintf("https://wiki.parkrun.com/index.php/%s", strings.ReplaceAll(event.Name, " ", "_"))
}

type Link struct {
	Name string
	Url  string
}

func (link Link) IsValid() bool {
	return len(link.Name) > 0 && len(link.Url) > 0
}

type ParkrunInfo struct {
	Id          string
	Name        string
	City        string
	Location    string
	RouteType   string
	RouteId     string
	GoogleMaps  string
	First       string
	Status      string
	Coordinates string
	Cafe        struct {
		Name       string
		GoogleMaps string
	}
	Strava []Link
	Social []Link
}

func (info ParkrunInfo) ParseCoordinates() (Coordinates, error) {
	if info.Coordinates == "" {
		return InvalidCoordinates, nil
	}
	r := regexp.MustCompile(`^\s*(-?[0-9\.]+)\s+(-?[0-9\.]+)\s*$`)
	if m := r.FindStringSubmatch(info.Coordinates); m != nil {
		lat, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", info.Coordinates)
		}
		lon, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", info.Coordinates)
		}
		return Coordinates{lat, lon}, nil
	}
	return InvalidCoordinates, fmt.Errorf("cannot parse coordinates: %s", info.Coordinates)
}

var parkrun_infos map[string]*ParkrunInfo

func (event Event) FixedLocation() string {
	if info, ok := parkrun_infos[event.Id]; ok {
		if info.City != "" {
			return info.City
		}
	}

	return event.Location
}

func (event Event) GoogleMapsUrl() string {
	if info, ok := parkrun_infos[event.Id]; ok {
		if info.GoogleMaps != "" {
			return info.GoogleMaps
		}
	}

	return fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%f%%2C%f", event.Coords.Lat, event.Coords.Lon)
}

func (event Event) GoogleMapsCourseUrl() string {
	if event.GoogleMapsId != "" {
		return fmt.Sprintf("https://www.google.com/maps/d/viewer?mid=%s", event.GoogleMapsId)
	}
	return ""
}

func (event Event) Cafe() Link {
	if info, ok := parkrun_infos[event.Id]; ok {
		return Link{info.Cafe.Name, info.Cafe.GoogleMaps}
	}
	return Link{}
}

func (event Event) Strava() []Link {
	if info, ok := parkrun_infos[event.Id]; ok {
		return info.Strava
	}

	return nil
}

func (event Event) Social() []Link {
	if info, ok := parkrun_infos[event.Id]; ok {
		links := make([]Link, 0, len(info.Social))
		for _, link := range info.Social {
			if link.IsValid() {
				links = append(links, link)
			}
		}
		return links
	}

	return nil
}

func (event Event) FixedName() string {
	if info, ok := parkrun_infos[event.Id]; ok {
		if info.Name != "" {
			return info.Name
		}
	}

	return event.Name
}

func (event Event) First() string {
	if info, ok := parkrun_infos[event.Id]; ok {
		if info.First != "" {
			return info.First
		}
	}

	return "?"
}

func LoadEvents(events_json_file string, parkruns_json_file string, germanyOnly bool) ([]*Event, error) {
	buf1, err := utils.ReadFile(parkruns_json_file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", parkruns_json_file, err)
	}
	var infos []ParkrunInfo
	if err := json.Unmarshal(buf1, &infos); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", parkruns_json_file, err)
	}
	parkrun_infos = make(map[string]*ParkrunInfo)
	for _, info := range infos {
		parkrun_infos[info.Id] = &ParkrunInfo{info.Id, info.Name, info.City, info.Location, info.RouteType, info.RouteId, info.GoogleMaps, info.First, info.Status, info.Coordinates, info.Cafe, info.Strava, info.Social}
	}

	buf, err := utils.ReadFile(events_json_file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", events_json_file, err)
	}

	eventsJson, err := parkrunparser.ParseEvents(buf)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", events_json_file, err)
	}

	eventMap := make(map[string]*Event)
	eventList := make([]*Event, 0)
	for _, e := range eventsJson.Events {
		if germanyOnly && e.Country.Name() != "Germany" {
			continue
		}

		event := &Event{e.Name, e.LongName, e.Location, "", Coordinates{e.Coordinates.Lat, e.Coordinates.Lng}, e.Country.Url, "", "", nil, ""}
		eventList = append(eventList, event)
		eventMap[e.Name] = event
	}

	for _, info := range parkrun_infos {
		coordinates, err := info.ParseCoordinates()
		if err != nil {
			return nil, fmt.Errorf("when parsing coordinates of '%s': %v", info.Name, info.Coordinates)
		}
		if event, found := eventMap[info.Id]; found {
			if coordinates.IsValid() {
				event.Coords = coordinates
			}
			event.Status = info.Status
			event.SpecificLocation = info.Location
			event.RouteType = info.RouteType
			event.GoogleMapsId = info.RouteId
			continue
		}
		event := &Event{info.Id, info.Name, info.City, info.Location, coordinates, "", info.RouteId, info.RouteType, nil, info.Status}
		eventList = append(eventList, event)
	}

	sort.Slice(eventList, func(i, j int) bool {
		return eventList[i].Id < eventList[j].Id
	})
	return eventList, nil
}

type KML struct {
	Placemarks []Placemark `xml:"Document>Folder>Placemark"`
}

type Placemark struct {
	Name       string     `xml:"name"`
	Point      Point      `xml:"Point"`
	LineString LineString `xml:"LineString"`
}

type ExtendedData struct {
	Data []Data `xml:"Data"`
}

type Data struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value"`
}

type Point struct {
	Coordinates string `xml:"coordinates"`
}

type LineString struct {
	Coordinates string `xml:"coordinates"`
}

var reCoordinatesStart = regexp.MustCompile(`^\s*<coordinates>\s*$`)
var reCoordinatesEnd = regexp.MustCompile(`^\s*</coordinates>\s*$`)

func (event *Event) LoadKML(filePath string) error {
	buf, err := utils.ReadFile(filePath)
	if err != nil {
		return err
	}

	event.Tracks = make([][]Coordinates, 0)
	track := make([]Coordinates, 0)

	in := false
	for _, line := range strings.Split(string(buf), "\n") {
		if !in {
			if reCoordinatesStart.MatchString(line) {
				in = true
				track = make([]Coordinates, 0)
			}
		} else {
			if reCoordinatesEnd.MatchString(line) {
				in = false
				if len(track) > 1 {
					event.Tracks = append(event.Tracks, track)
				}
				track = make([]Coordinates, 0)
			} else {
				line = strings.TrimSpace(line)
				if len(line) == 0 {
					continue
				}
				c := strings.Split(line, ",")
				if len(c) != 3 {
					return fmt.Errorf("error parsing coordinates '%s': not 3 elements", line)
				}
				lon, err := strconv.ParseFloat(c[0], 64)
				if err != nil {
					return fmt.Errorf("error parsing coordinates '%s': %v", line, err)
				}
				lat, err := strconv.ParseFloat(c[1], 64)
				if err != nil {
					return fmt.Errorf("error parsing coordinates '%s': %v", line, err)
				}
				track = append(track, Coordinates{lat, lon})
			}
		}
	}

	if len(track) > 0 {
		return fmt.Errorf("unterminated coordinates list")
	}

	allowedPoints := 100
	initialPrecision := 0.00001
	deltaPrecision := 0.000001
	simplified := make([][]Coordinates, 0, len(event.Tracks))
	for _, track := range event.Tracks {
		if len(track) > allowedPoints {
			precision := initialPrecision
			s := make([][]float64, 0, len(track))
			for _, c := range track {
				s = append(s, []float64{c.Lat, c.Lon})
			}
			for len(s) > allowedPoints {
				s = simplifier.Simplify(s, precision, true)
				precision += deltaPrecision
			}
			track = track[:0]
			for _, c := range s {
				track = append(track, Coordinates{c[0], c[1]})
			}
			simplified = append(simplified, track)
		} else {
			simplified = append(simplified, track)
		}
	}

	event.Tracks = simplified

	return nil
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}

func RenderJs(events []*Event, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0770); err != nil {
		return err
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	fmt.Fprintf(out, "var parkruns = [\n")
	for i, event := range events {
		if i != 0 {
			fmt.Fprintf(out, ",\n")
		}
		fmt.Fprintf(out, "{\n")
		fmt.Fprintf(out, "\"id\": \"%s\",\n", event.Id)
		fmt.Fprintf(out, "\"url\": \"%s\",\n", event.Url())
		fmt.Fprintf(out, "\"name\": \"%s\",\n", escapeQuotes(event.Name))
		fmt.Fprintf(out, "\"lat\": %.5f, \"lon\": %f,\n", event.Coords.Lat, event.Coords.Lon)
		fmt.Fprintf(out, "\"location\": \"%s\",\n", escapeQuotes(event.FixedLocation()))
		fmt.Fprintf(out, "\"googleMapsUrl\": \"%s\",\n", event.GoogleMapsUrl())
		fmt.Fprintf(out, "\"tracks\": [")
		for it, track := range event.Tracks {
			if it != 0 {
				fmt.Fprintf(out, ",")
			}
			fmt.Fprintf(out, "[")
			for ic, coord := range track {
				if ic != 0 {
					fmt.Fprintf(out, ",")
				}
				fmt.Fprintf(out, "[%.5f,%.5f]", coord.Lat, coord.Lon)
			}
			fmt.Fprintf(out, "]")
		}
		fmt.Fprintf(out, "],\n")

		if event.Active() {
			fmt.Fprintf(out, "\"active\": true,\n")
		} else {
			fmt.Fprintf(out, "\"active\": false,\n")
		}
		if event.Planned() {
			fmt.Fprintf(out, "\"planned\": true,\n")
		} else {
			fmt.Fprintf(out, "\"planned\": false,\n")
		}

		/*
		   "tracks" : "{{.EncodedTracks}}",

		*/
		fmt.Fprintf(out, "}\n")
	}
	fmt.Fprintf(out, "];")

	return nil
}
