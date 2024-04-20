package parkrun

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flopp/parkrun-map/internal/utils"
	simplifier "github.com/yrsh/simplify-go"
)

const (
	SEX_UNKNOWN = iota
	SEX_FEMALE
	SEX_MALE
)

type Participant struct {
	Id       string
	Name     string
	AgeGroup string
	Sex      int
	Runs     int64
	Vols     int64
	Time     time.Duration
}

var reAgeGroup1 = regexp.MustCompile(`^[A-Z]([fFmMwW])(\d+-\d+)$`)
var reAgeGroup2 = regexp.MustCompile(`^[A-Z]([fFmMwW])(\d+)$`)
var reAgeGroup3 = regexp.MustCompile(`^([fFmMwW])(WC)$`)

func ParseAgeGroup(s string) (string, int, error) {
	if s == "" {
		return "??", SEX_UNKNOWN, nil
	}
	if match := reAgeGroup1.FindStringSubmatch(s); match != nil {
		if match[1] == "f" || match[1] == "F" || match[1] == "w" || match[1] == "W" {
			return match[2], SEX_FEMALE, nil
		}
		return match[2], SEX_MALE, nil
	}
	if match := reAgeGroup2.FindStringSubmatch(s); match != nil {
		if match[1] == "f" || match[1] == "F" || match[1] == "w" || match[1] == "W" {
			return match[2], SEX_FEMALE, nil
		}
		return match[2], SEX_MALE, nil
	}
	if match := reAgeGroup3.FindStringSubmatch(s); match != nil {
		if match[1] == "f" || match[1] == "F" || match[1] == "w" || match[1] == "W" {
			return match[2], SEX_FEMALE, nil
		}
		return match[2], SEX_MALE, nil
	}

	return s, SEX_UNKNOWN, fmt.Errorf("unknown age group: %s", s)
}

type Results struct {
	Index int
	Date  time.Time

	Runners []*Participant
}

type Run struct {
	Event       *Event
	Index       int
	Date        time.Time
	RunnerCount int
	Results     *Results
}

func (run Run) Url() string {
	return fmt.Sprintf("https://parkrun.com.de/%s/results/%d/", run.Event.Id, run.Index)
}

func (run Run) DateF() string {
	return run.Date.Format("02.01.2006")
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d = d - m*time.Minute
	s := d / time.Second

	if h == 0 {
		return fmt.Sprintf("%02d:%02d", m, s)
	}
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

func (run Run) FastestT() string {
	if run.Results != nil && len(run.Results.Runners) > 0 {
		return fmtDuration(run.Results.Runners[0].Time)
	}
	return "-"
}

var patternDateIndex = regexp.MustCompile(`<h3><span class="format-date">([^<]+)</span><span class="spacer">[^<]*</span><span>#([0-9]+)</span></h3>`)
var patternRunnerRow0 = regexp.MustCompile(`<tr class="Results-table-row" [^<]*><td class="Results-table-td Results-table-td--position">\d+</td><td class="Results-table-td Results-table-td--name"><div class="compact">(<a href="[^"]*/\d+")?.*?</tr>`)
var patternRunnerRow = regexp.MustCompile(`^<tr class="Results-table-row" data-name="([^"]*)" data-agegroup="([^"]*)" data-club="[^"]*" data-gender="[^"]*" data-position="\d+" data-runs="(\d+)" data-vols="(\d+)" data-agegrade="[^"]*" data-achievement="([^"]*)"><td class="Results-table-td Results-table-td--position">\d+</td><td class="Results-table-td Results-table-td--name"><div class="compact"><a href="[^"]*/(\d+)"`)
var patternRunnerRowUnknown = regexp.MustCompile(`^<tr class="Results-table-row" data-name="([^"]*)" data-agegroup="" data-club="" data-position="\d+" data-runs="0" data-agegrade="0" data-achievement=""><td class="Results-table-td Results-table-td--position">\d+</td><td class="Results-table-td Results-table-td--name"><div class="compact">.*`)
var patternTime = regexp.MustCompile(`Results-table-td--time[^"]*&#10;                      "><div class="compact">(\d?:?\d\d:\d\d)</div>`)

//var patternVolunteerRow = regexp.MustCompile(`<a href='\./athletehistory/\?athleteNumber=(\d+)'>([^<]+)</a>`)

func (run *Run) LoadResults(filePath string) error {
	buf, err := utils.ReadFile(filePath)
	if err != nil {
		return err
	}
	reNewline := regexp.MustCompile(`\r?\n`)
	sbuf := reNewline.ReplaceAllString(string(buf), " ")

	results := Results{}

	if matchIndex := patternDateIndex.FindStringSubmatch(sbuf); matchIndex == nil {
		return fmt.Errorf("cannot find run index")
	} else {
		s := matchIndex[2]
		index, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("cannot parse index: %v", err)
		}
		results.Index = index
	}

	matchesR0 := patternRunnerRow0.FindAllStringSubmatch(sbuf, -1)
	for _, match0 := range matchesR0 {
		if match := patternRunnerRow.FindStringSubmatch(match0[0]); match != nil {
			name := html.UnescapeString(match[1])

			ageGroup, sex, err := ParseAgeGroup(match[2])
			if err != nil {
				return err
			}

			runs, err := strconv.Atoi(match[3])
			if err != nil {
				return err
			}

			vols, err := strconv.Atoi(match[4])
			if err != nil {
				return err
			}
			id := match[6]

			var runTime time.Duration = 0
			if matchTime := patternTime.FindStringSubmatch(match0[0]); matchTime != nil {
				split := strings.Split(matchTime[1], ":")
				if len(split) == 3 {
					t, err := time.ParseDuration(fmt.Sprintf("%sh%sm%ss", split[0], split[1], split[2]))
					if err != nil {
						panic(err)
					}
					runTime = t
				} else if len(split) == 2 {
					t, err := time.ParseDuration(fmt.Sprintf("%sm%ss", split[0], split[1]))
					if err != nil {
						panic(err)
					}
					runTime = t
				} else {
					panic(fmt.Errorf("cannot parse duration: %s", matchTime[1]))
				}
			}

			results.Runners = append(results.Runners, &Participant{id, name, ageGroup, sex, int64(runs), int64(vols), runTime})
			continue
		}

		if match := patternRunnerRowUnknown.FindStringSubmatch(match0[0]); match != nil {
			name := html.UnescapeString(match[1])
			results.Runners = append(results.Runners, &Participant{"", name, "??", SEX_UNKNOWN, 0, 0, 0})
			continue
		}

		return fmt.Errorf("cannot parse table row: %s", match0[0])
	}

	var runnerWithTime *Participant = nil
	for _, p := range results.Runners {
		if p.Time != 0 {
			runnerWithTime = p
			break
		}
	}
	if runnerWithTime != nil {
		for _, p := range results.Runners {
			if p.Time != 0 {
				runnerWithTime = p
			} else {
				p.Time = runnerWithTime.Time
			}
		}
	}

	run.Results = &results

	return nil
}

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
	EventId      int
	Id           string
	Name         string
	Location     string
	Coords       Coordinates
	CountryUrl   string
	GoogleMapsId string
	Tracks       [][]Coordinates
	LatestRun    *Run
	Order        int
	Status       string
}

func (event Event) Active() bool {
	return event.Status == ""
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

func (event Event) ReportUrl() string {
	return fmt.Sprintf("https://results-service.parkrun.com/resultsSystem/App/eventJournoReportHTML.php?evNum=%d", event.EventId)
}

func (event Event) LastRun() string {
	run := event.LatestRun
	if run == nil {
		return "n/a"
	}
	return fmt.Sprintf("#%d am %s mit %d Teilnehmern", run.Index, run.Date.Format("01.02.2006"), run.RunnerCount)
}

type Link struct {
	Name string
	Url  string
}

type ParkrunInfo struct {
	Id          string
	Name        string
	City        string
	GoogleMaps  string
	First       string
	Status      string
	Coordinates string
	Strava      []Link
	Social      []Link
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

func (event Event) Strava() []Link {
	if info, ok := parkrun_infos[event.Id]; ok {
		return info.Strava
	}

	return nil
}

func (event Event) Social() []Link {
	if info, ok := parkrun_infos[event.Id]; ok {
		return info.Social
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
		return nil, err
	}
	var infos []ParkrunInfo
	if err := json.Unmarshal(buf1, &infos); err != nil {
		return nil, err
	}
	parkrun_infos = make(map[string]*ParkrunInfo)
	for _, info := range infos {
		parkrun_infos[info.Id] = &ParkrunInfo{info.Id, info.Name, info.City, info.GoogleMaps, info.First, info.Status, info.Coordinates, info.Strava, info.Social}
	}

	buf, err := utils.ReadFile(events_json_file)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, err
	}

	countriesI, ok := result["countries"]
	if !ok {
		return nil, fmt.Errorf("cannot get 'countries' from 'events.json")
	}
	countries := countriesI.(map[string]interface{})

	countryLookup := make(map[string]string)
	for countryId, countryI := range countries {
		country := countryI.(map[string]interface{})

		urlI, ok := country["url"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'countries/%s/url' from 'events.json", countryId)
		}

		if urlI != nil {
			countryLookup[countryId] = urlI.(string)
		}
	}

	eventsI, ok := result["events"]
	if !ok {
		return nil, fmt.Errorf("cannot get 'events' from 'events.json")
	}
	events := eventsI.(map[string]interface{})

	featuresI, ok := events["features"]
	if !ok {
		return nil, fmt.Errorf("cannot get 'events/features' from 'events.json")
	}

	eventMap := make(map[string]*Event)
	eventList := make([]*Event, 0)
	features := featuresI.([]interface{})
	for _, featureI := range features {
		feature := featureI.(map[string]interface{})

		idI, ok := feature["id"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/id' from 'events.json")
		}
		id := int(idI.(float64))

		propertiesI, ok := feature["properties"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/properties' from 'events.json")
		}

		properties := propertiesI.(map[string]interface{})
		nameI, ok := properties["eventname"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/properties/eventname' from 'events.json")
		}
		name := nameI.(string)
		longNameI, ok := properties["EventLongName"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/properties/EventLongName' from 'events.json")
		}
		countryCodeI, ok := properties["countrycode"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/properties/countrycode' from 'events.json")
		}
		locationI, ok := properties["EventLocation"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/properties/EventLocation' from 'events.json")
		}
		longName := longNameI.(string)
		location := locationI.(string)
		countryCode := fmt.Sprintf("%.0f", countryCodeI.(float64))
		if germanyOnly && countryCode != "32" {
			continue
		}
		countryUrl, ok := countryLookup[countryCode]
		if !ok {
			return nil, fmt.Errorf("cannot lookup country code '%s' for '%s'", countryCode, name)
		}

		geometryI, ok := feature["geometry"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/geometry' from 'events.json")
		}
		geometry := geometryI.(map[string]interface{})
		coordinatesI, ok := geometry["coordinates"]
		if !ok {
			return nil, fmt.Errorf("cannot get 'events/features/geometry/coordinates' from 'events.json")
		}
		coordinates, ok := coordinatesI.([]interface{})
		if !ok || len(coordinates) != 2 {
			return nil, fmt.Errorf("bad length %d of 'events/features/geometry/coordinates' from 'events.json", len(coordinates))
		}
		lat := coordinates[1].(float64)
		lon := coordinates[0].(float64)

		event := &Event{id, name, longName, location, Coordinates{lat, lon}, countryUrl, "", nil, nil, 0, ""}
		eventList = append(eventList, event)
		eventMap[name] = event
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
			continue
		}
		event := &Event{0, info.Id, info.Name, info.City, coordinates, "", "", nil, nil, 0, info.Status}
		eventList = append(eventList, event)
	}

	sort.Slice(eventList, func(i, j int) bool {
		return eventList[i].Id < eventList[j].Id
	})
	return eventList, nil
}

var reDate = regexp.MustCompile(`^\s*(\d+)(st|nd|rd|th)\s+(\S+)\s+(\d\d\d\d)\s*$`)

func parseDate(s string) (time.Time, error) {
	m := reDate.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, fmt.Errorf("regexp failed")
	}

	day, err := strconv.ParseInt(m[1], 10, 0)
	if err != nil {
		return time.Time{}, err
	}
	year, err := strconv.ParseInt(m[4], 10, 0)
	if err != nil {
		return time.Time{}, err
	}
	for month := 1; month <= 12; month += 1 {
		if m[3] == time.Month(month).String() {
			return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse month: %s", m[3])
}

var reLine1 = regexp.MustCompile(`<body><h1>(.*)<br />Event number ([0-9]+)<br />(.*)</h1>`)
var reLine2 = regexp.MustCompile(`<p>This week ([0-9]+) people`)

func (event *Event) LoadReport(filePath string) error {
	buf, err := utils.ReadFile(filePath)
	if err != nil {
		return err
	}
	sbuf := string(buf)

	match := reLine1.FindStringSubmatch(sbuf)
	if match == nil {
		return fmt.Errorf("cannot fine line1 pattern")
	}

	runIndex, err := strconv.ParseInt(match[2], 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse run index: %s", match[2])
	}

	date, err := parseDate(match[3])
	if err != nil {
		return fmt.Errorf("cannot parse run date: %s; %s", match[3], err)
	}

	match = reLine2.FindStringSubmatch(sbuf)
	if match == nil {
		return fmt.Errorf("cannot fine line2 pattern")
	}
	runners, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse runners: %s", match[2])
	}

	event.LatestRun = &Run{event, int(runIndex), date, int(runners), nil}
	return nil
}

const (
	StateStart = iota
	StateDate
	StateIndex
	StateRunners
	StateEnd
)

func (event *Event) LoadWiki(filePath string) error {
	buf, err := utils.ReadFile(filePath)
	if err != nil {
		return err
	}
	sbuf := string(buf)

	state := StateStart
	dateS := ""
	indexS := ""
	runnersS := ""
	reTd := regexp.MustCompile(`^\s*<td>(.*)\s*$`)
	for _, line := range strings.Split(sbuf, "\n") {
		if state == StateStart {
			if strings.Contains(line, "Most_Recent_Event_Summary") {
				state = StateDate
			}
		} else if state == StateDate {
			if m := reTd.FindStringSubmatch(line); m != nil {
				dateS = strings.TrimSpace(m[1])
				state = StateIndex
			}
		} else if state == StateIndex {
			if m := reTd.FindStringSubmatch(line); m != nil {
				indexS = strings.TrimSpace(m[1])
				state = StateRunners
			}
		} else if state == StateRunners {
			if m := reTd.FindStringSubmatch(line); m != nil {
				runnersS = strings.TrimSpace(m[1])
				state = StateEnd
				break
			}
		}
	}
	if state != StateEnd {
		return fmt.Errorf("failed to fetch results table")
	}

	if dateS == "" {
		// no runs, yet!
		return nil
	}

	date, err := parseDate(dateS)
	if err != nil {
		return fmt.Errorf("cannot parse run date: %s; %s", dateS, err)
	}
	index, err := strconv.ParseInt(indexS, 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse run index: %s", indexS)
	}
	runners, err := strconv.ParseInt(runnersS, 10, 32)
	if err != nil {
		return fmt.Errorf("cannot parse runners: %s", runnersS)
	}

	event.LatestRun = &Run{event, int(index), date, int(runners), nil}
	return nil
}

// <iframe src="https://www.google.com/maps/d/embed?t=h&mid=1jzu9KWQBw__FbZHD3RW6KqLY9CxMzQAa" width="450" height="450"></iframe>
var reMapsId = regexp.MustCompile(`<iframe src="https://www\.google\.[^/]+/maps/[^"]*mid=([^"&]+)("|&)`)

func (event *Event) LoadCoursePage(filePath string) error {
	buf, err := utils.ReadFile(filePath)
	if err != nil {
		return err
	}
	sbuf := string(buf)

	event.GoogleMapsId = ""
	for _, line := range strings.Split(sbuf, "\n") {
		if m := reMapsId.FindStringSubmatch(line); m != nil {
			event.GoogleMapsId = m[1]
		}
	}

	if event.GoogleMapsId == "" {
		return fmt.Errorf("cannot find map of course page")
	}
	return nil
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

func escape(s string) string {
	return strings.ReplaceAll(s, "\\", "\\\\")
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
		fmt.Fprintf(out, "\"location\": \"%s\",\n", escapeQuotes(event.Location))
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
		if event.LatestRun != nil {
			fmt.Fprintf(out, "\"latest\": {\n")
			fmt.Fprintf(out, "\"index\": %d,\n", event.LatestRun.Index)
			fmt.Fprintf(out, "\"date\": \"%s\",\n", event.LatestRun.DateF())
			fmt.Fprintf(out, "\"runners\": %d\n", event.LatestRun.RunnerCount)
			fmt.Fprintf(out, "}\n")
		} else {
			fmt.Fprintf(out, "\"latest\": null\n")
		}

		/*
		   "tracks" : "{{.EncodedTracks}}",

		*/
		fmt.Fprintf(out, "}\n")
	}
	fmt.Fprintf(out, "];")

	return nil
}
