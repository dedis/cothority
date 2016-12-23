package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type JSON struct {
	Schedule Schedule
}

type Schedule struct {
	Conference Conference
}

type Conference struct {
	Days []Day
}

type Day struct {
	Rooms map[string][]Track
}

type Track struct {
	Id       int
	Duration string
	Persons  []Person
	Date     string
	Room     string
	Title    string
}

type Person struct {
	Public string `json:"public_name"`
}

// custom database yay
type database_ struct {
	db map[int]Entry_
	sync.Mutex
}

// Entry_ represents any conferences at 33c3
type Entry_ struct {
	Name     string
	Duration string
	Persons  string
	Date     string
	Room     string
	// map of tag => vote status
	Votes map[string]bool
}
type EntryJSON struct {
	Id       int
	Name     string
	Persons  string
	Room     string
	Date     string
	Duration string
	Up       int
	Down     int
	Voted    bool
}

type EntriesJSON struct {
	Entries []EntryJSON
}

func (ej *EntriesJSON) Len() int {
	return len(ej.Entries)
}

func (ej *EntriesJSON) Less(i, j int) bool {
	return ej.Entries[i].Name < ej.Entries[j].Name
}

func (ej *EntriesJSON) Swap(i, j int) {
	ej.Entries[i], ej.Entries[j] = ej.Entries[j], ej.Entries[i]
}

func newDatabase() *database_ {
	return &database_{db: map[int]Entry_{}}
}

// Returns the JSON representation with information including whether this tag
// has voted or not
func (d *database_) JSON(tag string, update bool) ([]byte, error) {
	d.Lock()
	defer d.Unlock()

	var entriesJSON []EntryJSON
	// list of entries
	for id, entry := range d.db {
		_, voted := entry.Votes[tag]
		var up, down = 0, 0
		// count the votes
		for _, v := range entry.Votes {
			if v {
				up++
			} else {
				down++
			}
		}
		eJson := EntryJSON{
			Id:    id,
			Up:    up,
			Down:  down,
			Voted: voted,
		}
		if !update {
			eJson.Name = entry.Name
			eJson.Persons = entry.Persons
			eJson.Duration = entry.Duration
			eJson.Date = entry.Date
			eJson.Room = entry.Room
		}
		entriesJSON = append(entriesJSON, eJson)
	}
	var ej = &EntriesJSON{entriesJSON}
	sort.Stable(ej)
	return json.Marshal(ej.Entries)
}

func (d *database_) VoteOrError(id int, tag string, vote bool) error {
	d.Lock()
	defer d.Unlock()
	e, ok := d.db[id]
	if !ok {
		return errors.New("invalid entry id")
	}
	if v, ok := e.Votes[tag]; ok && v == vote {
		return errors.New("users already voted")
	}
	e.Votes[tag] = vote
	return nil
}

func (d *database_) load(fileName string) {
	d.Lock()
	defer d.Unlock()
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}

	var j JSON
	if err := json.NewDecoder(file).Decode(&j); err != nil {
		panic(err)
	}

	conf := j.Schedule.Conference
	var count int
	for _, day := range conf.Days {
		for _, dayTracks := range day.Rooms {
			for _, t := range dayTracks {
				count++
				var personStr []string
				for _, p := range t.Persons {
					personStr = append(personStr, p.Public)
				}
				date, err := time.Parse(time.RFC3339, t.Date)
				if err != nil {
					fmt.Printf("[-] Could not parse date %d: %s\n", t.Date, t.Title)
					continue
				}
				formattedDate := fmt.Sprintf("%02d/%02d %02d:%02d", date.Day(),
					date.Month(),
					date.Hour(),
					date.Minute())
				d.db[t.Id] = Entry_{
					Name:     t.Title,
					Date:     formattedDate,
					Persons:  strings.Join(personStr, ","),
					Duration: t.Duration,
					Room:     t.Room,
					Votes:    make(map[string]bool),
				}
			}
		}
	}
	fmt.Println("[+] Loaded ", count, " tracks")
}

func (t *Track) String() string {
	return fmt.Sprintf("%d: %s (%s in %s)", t.Id, t.Title, t.Duration, t.Room)
}
