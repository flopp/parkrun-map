package parkrun

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flopp/parkrun-map/internal/utils"
)

type Run struct {
	Event       *Event
	Index       int
	Date        time.Time
	RunnerCount int
}

func (run Run) Url() string {
	return fmt.Sprintf("https://parkrun.com.de/%s/results/%d/", run.Event.Id, run.Index)
}

func (run Run) DateF() string {
	return run.Date.Format("02.01.2006")
}

type Coordinates struct {
	Lat, Lon float64
}

type Event struct {
	EventId      int
	Id           string
	Name         string
	Location     string
	Lat          float64
	Lon          float64
	GoogleMapsId string
	Tracks       [][]Coordinates
	LatestRun    *Run
}

func (event Event) Url() string {
	return fmt.Sprintf("https://parkrun.com.de/%s", event.Id)
}

func (event Event) CoursePageUrl() string {
	return fmt.Sprintf("https://parkrun.com.de/%s/course", event.Id)
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

var locations map[string]string

func (event Event) FixedLocation() string {
	if locations == nil {
		locations = make(map[string]string)

		locations["aachenerweiher"] = "Köln"
		locations["allerpark"] = "Wolfsburg"
		locations["alstervorland"] = "Hamburg"
		locations["bahnstadtpromenade"] = "Heidelberg"
		locations["bugasee"] = "Kassel"
		locations["dietenbach"] = "Freiburg"
		locations["dreilaendergarten"] = "Weil am Rhein"
		locations["ebenberg"] = "Landau in der Pfalz"
		locations["ehrenbreitstein"] = "Koblenz"
		locations["emmerwiesen"] = "Bad Pyrmont"
		locations["friedrichsau"] = "Ulm"
		locations["fuldaaue"] = "Fulda"
		locations["georgengarten"] = "Hannover"
		locations["globe"] = "Schwäbisch Hall"
		locations["gruenerweg"] = "Bad Urach"
		locations["hasenheide"] = "Berlin"
		locations["havelkanal"] = "Hennigsdorf"
		locations["hockgraben"] = "Konstanz"
		locations["kastanienallee"] = "Tübingen"
		locations["kemnadersee"] = "Bochum"
		locations["kiessee"] = "Göttingen"
		locations["kraeherwald"] = "Stuttgart"
		locations["kuechenholz"] = "Leipzig"
		locations["kurtschumacherpromenade"] = "Würzburg"
		locations["lahnwiesen"] = "Marburg"
		locations["landesgartenschaupark"] = "Neumarkt in der Oberpfalz"
		locations["leinpfad"] = "Merzig"
		locations["lousberg"] = "Aachen"
		locations["luitpold"] = "Ingolstadt"
		locations["maaraue"] = "Mainz"
		locations["mattheiserweiher"] = "Trier"
		locations["monrepos"] = "Ludwigsburg"
		locations["nidda"] = "Frankfurt am Main"
		locations["neckarau"] = "Mannheim"
		locations["neckaruferesslingen"] = "Esslingen"
		locations["obersee"] = "Bielefeld"
		locations["oberwald"] = "Karlsruhe"
		locations["offenthal"] = "Offenthal"
		locations["prestelsee"] = "Graben-Neudorf"
		locations["priessnitzgrund"] = "Dresden"
		locations["prinzenpark"] = "Braunschweig"
		locations["rheinaue"] = "Bonn"
		locations["rheinpark"] = "Köln"
		locations["riemer"] = "München"
		locations["rosensteinpark"] = "Stuttgart"
		locations["rubbenbruchsee"] = "Osnabrück"
		locations["seewoog"] = "Ramstein-Miesenbach"
		locations["schwanenteich"] = "Giessen"
		locations["speyerleinpfad"] = "Speyer"
		locations["sportparkrems"] = "Schorndorf"
		locations["stadtpark"] = "Fürth"
		locations["traumschleifebaerenbachpfad"] = "Baumholder"
		locations["unisee"] = "Bremen"
		locations["volksgarten"] = "Düsseldorf"
		locations["wertwiesen"] = "Heilbronn"
		locations["westpark"] = "München"
		locations["wienburgpark"] = "Münster"
		locations["woehrdersee"] = "Nürnberg"
		locations["ziegelwiese"] = "Halle (Saale)"
	}

	if location, ok := locations[event.Id]; ok {
		return location
	}

	return event.Location
}

var googleMapsUrls map[string]string

func (event Event) GoogleMapsUrl() string {
	if googleMapsUrls == nil {
		googleMapsUrls = make(map[string]string)

		googleMapsUrls["aachenerweiher"] = "https://maps.app.goo.gl/gktwZJVrqsirFma7A"
		googleMapsUrls["allerpark"] = "https://maps.app.goo.gl/viU3mCPcVfMiLb8RA"
		googleMapsUrls["alstervorland"] = "https://maps.app.goo.gl/nzFz8ds8yBCSPq9aA"
		googleMapsUrls["bahnstadtpromenade"] = "https://maps.app.goo.gl/y9xjkARDxHbwaRd3A"
		googleMapsUrls["bugasee"] = "https://maps.app.goo.gl/nQAYeuPuELrezZC49"
		googleMapsUrls["dietenbach"] = "https://maps.app.goo.gl/8mjy464YdjBKqmR1A"
		googleMapsUrls["dreilaendergarten"] = "https://maps.app.goo.gl/Ncezh996xdM6YnrV9"
		googleMapsUrls["ebenberg"] = "https://maps.app.goo.gl/v2oDppZt5VDkqc868"
		googleMapsUrls["ehrenbreitstein"] = "https://maps.app.goo.gl/VCdwKTUdxT8K7JvWA"
		googleMapsUrls["emmerwiesen"] = "https://maps.app.goo.gl/MRUt6XqQgeMErnXP9"
		googleMapsUrls["friedrichsau"] = "https://maps.app.goo.gl/DJJZCmyTZC2JT6ks8"
		googleMapsUrls["fuldaaue"] = "https://maps.app.goo.gl/ik8YRD92dfJ4EMRe7"
		googleMapsUrls["georgengarten"] = "https://maps.app.goo.gl/viMQE7RHzjhz5u8L7"
		googleMapsUrls["globe"] = "https://maps.app.goo.gl/4rv49Tm1inbFd76h8"
		googleMapsUrls["gruenerweg"] = "https://maps.app.goo.gl/nta1yjbphkXFtf4E9"
		googleMapsUrls["hasenheide"] = "https://maps.app.goo.gl/WBznzk2a3TKsjEFD6"
		googleMapsUrls["havelkanal"] = "https://maps.app.goo.gl/7TUSEr1KyoRxbmfQ8"
		googleMapsUrls["hockgraben"] = "https://maps.app.goo.gl/yGYerpCpG2gbG8nd9"
		googleMapsUrls["kastanienallee"] = "https://maps.app.goo.gl/iWmCZSdtF4TguKFGA"
		googleMapsUrls["kemnadersee"] = "https://maps.app.goo.gl/jAP4ZGfDcFScpHK89"
		googleMapsUrls["kiessee"] = "https://maps.app.goo.gl/dgFaKHnqUWrt5XFE6"
		googleMapsUrls["kraeherwald"] = "https://maps.app.goo.gl/SQtTyVHupKn5FDZn9"
		googleMapsUrls["kuechenholz"] = "https://maps.app.goo.gl/DgogwFVSrqU281Le6"
		googleMapsUrls["kurtschumacherpromenade"] = "https://maps.app.goo.gl/GKDejRda5ir2WcJv5"
		googleMapsUrls["lahnwiesen"] = "https://maps.app.goo.gl/DpW4Xuzr9VbGQAak9"
		googleMapsUrls["landesgartenschaupark"] = "https://maps.app.goo.gl/KvWPa2Q6yRED86jz9"
		googleMapsUrls["leinpfad"] = "https://maps.app.goo.gl/efT8fKk5bDpnvWbL7"
		googleMapsUrls["lousberg"] = "https://maps.app.goo.gl/ihumiC3xKAa16hHW9"
		googleMapsUrls["luitpold"] = "https://maps.app.goo.gl/w2UxEFGWBuhQq9X68"
		googleMapsUrls["maaraue"] = "https://maps.app.goo.gl/Tf5cqkQzHoFY8kHd7"
		googleMapsUrls["mattheiserweiher"] = "https://maps.app.goo.gl/FstemrgLn3XZFjJM9"
		googleMapsUrls["monrepos"] = "https://maps.app.goo.gl/oQUjxeNSkL2HymFe6"
		googleMapsUrls["nidda"] = "https://maps.app.goo.gl/fUvUWeqBp8eGbKBC7"
		googleMapsUrls["neckarau"] = "https://maps.app.goo.gl/y2mf1JWysMn9GvXy5"
		googleMapsUrls["neckaruferesslingen"] = "https://maps.app.goo.gl/tEqiJTSJQbGUWsFA9"
		googleMapsUrls["obersee"] = "https://maps.app.goo.gl/VjyTtqHZFTGVKBft6"
		googleMapsUrls["oberwald"] = "https://maps.app.goo.gl/VCS8PPaw2bUiKNWE9"
		googleMapsUrls["offenthal"] = "https://maps.app.goo.gl/dW7WQaySgG5ibDau6"
		googleMapsUrls["prestelsee"] = "https://maps.app.goo.gl/mVWfRAqjsD844Kg29"
		googleMapsUrls["priessnitzgrund"] = "https://maps.app.goo.gl/Cgw3ePYLSQDyszMCA"
		googleMapsUrls["prinzenpark"] = "https://maps.app.goo.gl/k9zPGx8kCegMLnGv9"
		googleMapsUrls["rheinaue"] = "https://maps.app.goo.gl/4Bd7qpAdVLnNY7ZW6"
		googleMapsUrls["rheinpark"] = "https://maps.app.goo.gl/oK7jMizoDhZYB1oB8"
		googleMapsUrls["riemer"] = "https://maps.app.goo.gl/fLJzPER422ezkUu78"
		googleMapsUrls["rosensteinpark"] = "https://maps.app.goo.gl/VzhGMisjcrWnm5WD8"
		googleMapsUrls["rubbenbruchsee"] = "https://maps.app.goo.gl/7NuAQshN5EHZ6phV8"
		googleMapsUrls["seewoog"] = "https://maps.app.goo.gl/3FBjKEi8M2Q6VzUG8"
		googleMapsUrls["schwanenteich"] = "https://maps.app.goo.gl/LE4RJQJokrC3kndR7"
		googleMapsUrls["speyerleinpfad"] = "https://maps.app.goo.gl/KgYAvV7jUTGeCYW1A"
		googleMapsUrls["sportparkrems"] = "https://maps.app.goo.gl/NeoaGX9k8WEhoPDTA"
		googleMapsUrls["stadtpark"] = "https://maps.app.goo.gl/AMUEyC33WRPzDybZA"
		googleMapsUrls["traumschleifebaerenbachpfad"] = "https://maps.app.goo.gl/oWZkfCJYRJ8nDfKD6"
		googleMapsUrls["unisee"] = "https://maps.app.goo.gl/GorNnFc7GRfLrBxU7"
		googleMapsUrls["volksgarten"] = "https://maps.app.goo.gl/4xX1KDT7zCbQbkXGA"
		googleMapsUrls["wertwiesen"] = "https://maps.app.goo.gl/zgxhJuvV14fcEroe6"
		googleMapsUrls["westpark"] = "https://maps.app.goo.gl/FhZErPa2jXv4esQs5"
		googleMapsUrls["wienburgpark"] = "https://maps.app.goo.gl/7gTE7B9EiWN9rDyL6"
		googleMapsUrls["woehrdersee"] = "https://maps.app.goo.gl/1u1SCvAM9LvBZhWy8"
		googleMapsUrls["ziegelwiese"] = "https://maps.app.goo.gl/m8aqqhcobHdKk61A6"
	}

	if url, ok := googleMapsUrls[event.Id]; ok {
		return url
	}

	return fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%f%%2C%f", event.Lat, event.Lon)
}

var names map[string]string

func (event Event) FixedName() string {
	if names == nil {
		names = make(map[string]string)

		names["bugasee"] = "Bugasee parkrun"
		names["neckaruferesslingen"] = "Neckarufer parkrun"
	}

	if name, ok := names[event.Id]; ok {
		return name
	}

	return event.Name
}

func LoadEvents(events_json_file string) ([]*Event, error) {
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
		if countryCode != "32" {
			continue
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

		eventList = append(eventList, &Event{id, name, longName, location, lat, lon, "", nil, nil})
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

	event.LatestRun = &Run{event, int(runIndex), date, int(runners)}
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

	event.LatestRun = &Run{event, int(index), date, int(runners)}
	return nil
}

// <iframe src="https://www.google.com/maps/d/embed?t=h&mid=1jzu9KWQBw__FbZHD3RW6KqLY9CxMzQAa" width="450" height="450"></iframe>
var reMapsId = regexp.MustCompile(`<iframe src="https://www.google.com/maps/[^"]*mid=([^"&]+)("|&)`)

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

	return nil
}
