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

type Event struct {
	EventId   int
	Id        string
	Name      string
	Location  string
	Lat       float64
	Lon       float64
	LatestRun *Run
}

func (event Event) Url() string {
	return fmt.Sprintf("https://parkrun.com.de/%s", event.Id)
}

func (event Event) WikiUrl() string {
	return fmt.Sprintf("https://wiki.parkrun.com/index.php/%s", strings.ReplaceAll(event.Name, " ", "_"))
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
		locations["havelkanal"] = "Henningsdorf bei Berlin"
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

		eventList = append(eventList, &Event{id, name, longName, location, lat, lon, nil})
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
	//<td>17th February 2024
	//<td>191
	//<td>43
	reTd := regexp.MustCompile(`^\s*<td>(.+)\s*$`)
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
