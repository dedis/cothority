package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type jsonSchedule struct {
	Schedule schedule
}

type schedule struct {
	Conference conference
}

type conference struct {
	Days []day
}

type day struct {
	Rooms map[string][]track
}

type track struct {
	ID       int
	Duration string
	Persons  []person
	Date     string
	Room     string
	Title    string
}

type person struct {
	Public string `json:"public_name"`
}

// custom database yay
type databaseStruct struct {
	DB map[int]*entryStruct
	sync.Mutex
}

// Entry_ represents any conferences at 33c3
type entryStruct struct {
	Name     string
	Duration string
	Persons  string
	Date     string
	Room     string
	// map of tag => vote status
	Votes []voteStruct
}

type voteStruct struct {
	Tag  []byte
	Vote bool
}

type entryJSON struct {
	ID       int
	Name     string
	Persons  string
	Room     string
	Date     string
	Duration string
	Up       int
	Down     int
	Voted    bool
}

type entriesJSON struct {
	Entries []entryJSON
}

func (ej *entriesJSON) Len() int {
	return len(ej.Entries)
}

func (ej *entriesJSON) Less(i, j int) bool {
	return ej.Entries[i].Name < ej.Entries[j].Name
}

func (ej *entriesJSON) Swap(i, j int) {
	ej.Entries[i], ej.Entries[j] = ej.Entries[j], ej.Entries[i]
}

func newDatabase() *databaseStruct {
	return &databaseStruct{DB: map[int]*entryStruct{}}
}

// Returns the JSON representation with information including whether this tag
// has voted or not
func (d *databaseStruct) JSON(tag []byte, update bool) ([]byte, error) {
	d.Lock()
	defer d.Unlock()

	var entriesJSON []entryJSON
	// list of entries
	for id, entry := range d.DB {
		var voted bool
		var up, down = 0, 0
		// count the votes
		for _, v := range entry.Votes {
			if bytes.Equal(tag, v.Tag) {
				voted = v.Vote
			}
			if v.Vote {
				up++
			} else {
				down++
			}
		}
		eJSON := entryJSON{
			ID:    id,
			Up:    up,
			Down:  down,
			Voted: voted,
		}
		if !update {
			eJSON.Name = entry.Name
			eJSON.Persons = entry.Persons
			eJSON.Duration = entry.Duration
			eJSON.Date = entry.Date
			eJSON.Room = entry.Room
		}
		entriesJSON = append(entriesJSON, eJSON)
	}
	var ej = &entriesJSON{entriesJSON}
	sort.Stable(ej)
	return json.Marshal(ej.Entries)
}

func (d *databaseStruct) VoteOrError(id int, tag []byte, vote bool) error {
	d.Lock()
	defer d.Unlock()
	e, ok := d.DB[id]
	if !ok {
		return errors.New("invalid entry id")
	}
	for i, t := range e.Votes {
		if bytes.Equal(tag, t.Tag) {
			if vote == t.Vote {
				return errors.New("users already voted")
			}
			e.Votes[i].Vote = vote
			return nil
		}
	}
	e.Votes = append(e.Votes, voteStruct{tag, vote})
	fmt.Println("Voted for ", e.Name, " from ", hex.EncodeToString(tag))
	return nil
}

func (d *databaseStruct) load(fileName string) {
	d.Lock()
	defer d.Unlock()
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}

	var j jsonSchedule
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
					fmt.Printf("[-] Could not parse date %s: %s\n", t.Date, t.Title)
					continue
				}
				formattedDate := fmt.Sprintf("%02d/%02d %02d:%02d", date.Day(),
					date.Month(),
					date.Hour(),
					date.Minute())
				d.DB[t.ID] = &entryStruct{
					Name:     t.Title,
					Date:     formattedDate,
					Persons:  strings.Join(personStr, ","),
					Duration: t.Duration,
					Room:     t.Room,
					Votes:    []voteStruct{},
				}
			}
		}
	}
	fmt.Println("[+] Loaded ", count, " tracks")
}

// VotesSave stores the votes for later usage
func (d *databaseStruct) VotesSave(fullName string) error {
	file, err := os.OpenFile(fullName, os.O_RDWR+os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(file).Encode(d); err != nil {
		return err
	}
	return file.Close()
}

// VotesLoad either loads the full database, including votes, or
// loads the database without votes. scheduleName is the plain
// json-database from the CCC-website, fullName is the database
// including the votes.
func (d *databaseStruct) VotesLoad(scheduleName, fullName string) error {
	d.load(scheduleName)
	_, err := os.Stat(fullName)
	if err != nil {
		return err
	}
	file, err := os.Open(fullName)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(d)
}

func (t *track) String() string {
	return fmt.Sprintf("%d: %s (%s in %s)", t.ID, t.Title, t.Duration, t.Room)
}
