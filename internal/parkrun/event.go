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

	"github.com/flopp/go-parkrunparser"
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

func (run Run) Runners() string {
	if run.RunnerCount == 0 {
		return "?"
	}
	return fmt.Sprintf("%d", run.RunnerCount)
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
	Id                          string
	Name                        string
	Location                    string
	SpecificLocation            string
	Coords                      Coordinates
	CountryUrl                  string
	GoogleMapsId                string
	RouteType                   string
	Tracks                      [][]Coordinates
	LatestRun                   *Run
	Current                     bool
	Order                       int
	Status                      string
	SummaryRegistrations        int
	SummaryRunners              int
	SummaryIndividualRunners    int
	SummaryVolunteers           int
	SummaryIndividualVolunteers int
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

func (event Event) LastRun() string {
	run := event.LatestRun
	if run == nil {
		return "n/a"
	}
	return fmt.Sprintf("#%d am %s mit %s Teilnehmern", run.Index, run.Date.Format("01.02.2006"), run.Runners())
}

func (event Event) SummaryRunnersAvg() string {
	if event.LatestRun != nil {
		return fmt.Sprintf("%.1f", float64(event.SummaryRunners)/float64(event.LatestRun.Index))
	}
	return "n/a"
}

func (event Event) SummaryVolunteersAvg() string {
	if event.LatestRun != nil {
		return fmt.Sprintf("%.1f", float64(event.SummaryVolunteers)/float64(event.LatestRun.Index))
	}
	return "n/a"
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

func (event Event) Outdated() bool {
	return !event.Current
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
		parkrun_infos[info.Id] = &ParkrunInfo{info.Id, info.Name, info.City, info.Location, info.RouteType, info.GoogleMaps, info.First, info.Status, info.Coordinates, info.Cafe, info.Strava, info.Social}
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

		event := &Event{e.Name, e.LongName, e.Location, "", Coordinates{e.Coordinates.Lat, e.Coordinates.Lng}, e.Country.Url, "", "", nil, nil, false, 0, "", 0, 0, 0, 0, 0}
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
			continue
		}
		event := &Event{info.Id, info.Name, info.City, info.Location, coordinates, "", "", info.RouteType, nil, nil, false, 0, info.Status, 0, 0, 0, 0, 0}
		eventList = append(eventList, event)
	}

	sort.Slice(eventList, func(i, j int) bool {
		return eventList[i].Id < eventList[j].Id
	})
	return eventList, nil
}

var reDate = regexp.MustCompile(`^\s*(\d+)(st|nd|rd|th)\s+(\S+)\s+(\d\d\d\d)\s*$`)
var reDate2 = regexp.MustCompile(`^\s*(\d\d)\.(\d\d)\.(\d\d\d\d)\s*$`)

func parseDate(s string) (time.Time, error) {
	dd := ""
	mm := ""
	yy := ""
	if m := reDate.FindStringSubmatch(s); m != nil {
		dd = m[1]
		yy = m[4]
		mm = m[3]
	} else if m := reDate2.FindStringSubmatch(s); m != nil {
		dd = m[1]
		mm = m[2]
		yy = m[3]
	} else {
		return time.Time{}, fmt.Errorf("cannot parse date (regexp failed): %s", s)
	}

	day, err := strconv.ParseInt(dd, 10, 0)
	if err != nil {
		return time.Time{}, err
	}
	year, err := strconv.ParseInt(yy, 10, 0)
	if err != nil {
		return time.Time{}, err
	}

	// named months
	for month := 1; month <= 12; month += 1 {
		if mm == time.Month(month).String() {
			return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.Local), nil
		}
	}
	// numbered months
	month, err := strconv.ParseInt(mm, 10, 0)
	if err == nil {
		return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("cannot parse date (month failed): %s", s)
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
	lines := strings.Split(sbuf, "\n")

	state := StateStart
	dateS := ""
	indexS := ""
	runnersS := ""
	reTd := regexp.MustCompile(`^\s*<td>(.*)\s*$`)
	for _, line := range lines {
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

	// try to find summary table
	table := make([]string, 0)
	state = 0
	for _, line := range lines {
		if state == 0 {
			if line == `<th>Past Week` {
				state = 1
				table = append(table, line)
			}
		} else if state == 1 {
			table = append(table, line)
			if line == `</table>` {
				state = 2
				break
			}
		}
	}
	if state == 2 {
		if err = event.parseWikiSummaryTable(table); err != nil {
			fmt.Printf("%s: while parsing wiki summary table from '%s': %v\n", event.Id, filePath, err)
		}
	}

	return nil
}

func (event *Event) parseWikiSummaryTable(lines []string) error {
	event.SummaryRegistrations = 0
	event.SummaryRunners = 0
	event.SummaryIndividualRunners = 0
	event.SummaryVolunteers = 0
	event.SummaryIndividualVolunteers = 0

	hasValue := false
	values := []*int{&event.SummaryRegistrations, &event.SummaryRunners, &event.SummaryIndividualRunners, &event.SummaryVolunteers, &event.SummaryIndividualVolunteers}
	headers := []string{"<th>Registrations", "<th>Runs", "<th>Participants", "<th>Volunteer Occasions", "<th>Volunteers", "</table>"}
	state := -1
	reTd := regexp.MustCompile(`^<td>(\d+)$`)

	for _, line := range lines {
		if line == headers[state+1] {
			state += 1
			hasValue = false
			if state+1 == len(headers) {
				break
			}
		} else if state >= 0 && !hasValue {
			if m := reTd.FindStringSubmatch(line); m != nil {
				hasValue = true
				if i, err := strconv.Atoi(m[1]); err != nil {
					return fmt.Errorf("cannot parse value: '%s'; error: %v", m[1], err)
				} else {
					*(values[state]) = i
				}
			}
		}
	}

	if state+1 != len(headers) {
		return fmt.Errorf("cannot find all fields")
	}

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
