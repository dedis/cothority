package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

////////////////////// HELPERS FUNCTIONS / STRUCT /////////////////
// StreamStats is used to compute the statistics
// it reprensent the time to an action (setup, shamir round, coll round etc)
// use it to compute streaming mean + dev
type StreamStats struct {
	min float64
	max float64

	n    int
	oldM float64
	newM float64
	oldS float64
	newS float64
	dev  float64
}

// Update will update the time struct with the min / max  change
// + compute new avg + new dev
// k is the number of times we've added something ("index" of the update)
// needed to compute the avg + dev
// streaming dev algo taken from http://www.johndcook.com/blog/standard_deviation/
func (t *StreamStats) Update(newTime float64) {
	// nothings takes 0 ms to complete, so we know it's the first time
	if t.min > newTime || t.n == 0 {
		t.min = newTime
	}
	if t.max < newTime {
		t.max = newTime
	}

	t.n += 1
	if t.n == 1 {
		t.oldM = newTime
		t.newM = newTime
		t.oldS = 0.0
	} else {
		t.newM = t.oldM + (newTime-t.oldM)/float64(t.n)
		t.newS = t.oldS + (newTime-t.oldM)*(newTime-t.newM)
		t.oldM = t.newM
		t.oldS = t.newS
	}
	t.dev = math.Sqrt(t.newS / float64(t.n-1))

}

// Average will set the current StreamStats to the average of all StreamStats
func StreamStatsAverage(st ...StreamStats) StreamStats {
	var t StreamStats
	for _, s := range st {
		t.min += s.min
		t.max += s.max
		t.newM += s.newM
		t.dev += s.dev
	}
	l := float64(len(st))
	t.min /= l
	t.max /= l
	t.newM /= l
	t.dev /= l
	t.n = len(st)
	return t
}

func (t *StreamStats) Min() float64 {
	return t.min
}
func (t *StreamStats) Max() float64 {
	return t.max
}

// return the number of value added
func (t *StreamStats) NumValue() int {
	return t.n
}

func (t *StreamStats) Avg() float64 {
	return t.newM
}

func (t *StreamStats) Dev() float64 {
	return t.dev
}

func (t *StreamStats) Header(prefix string) string {
	return fmt.Sprintf("%smin, %smax, %savg, %sdev", prefix, prefix, prefix, prefix)
}
func (t *StreamStats) String() string {
	return fmt.Sprintf("%f, %f, %f, %f", t.Min()/1e9, t.Max()/1e9, t.Avg()/1e9, t.Dev()/1e9)
}

////////////////////////////////////////////////////////

// generic typing of a Entry containing some timing data
type Entry interface{}

var ShamirRoundType string = "shamir_round"
var ShamirSetupType string = "shamir_setup"

// concrete impl
type ShamirEntry struct {
	Type  string  `json:"type"`
	Round int     `json:"round"`
	Time  float64 `json:"time"`
}

type CollServerEntry struct {
	App     string  `json:"eapp"`
	Host    string  `json:"ehost"`
	Level   string  `json:"elevel"`
	Msg     string  `json:"emsg"`
	MsgTime string  `json:"etime"`
	File    string  `json:"file"`
	Round   int     `json:"round"`
	Time    float64 `json:"time"`
	Type    string  `json:"type"`
}

type CollClientEntry struct {
	File        string    `json:"file"`
	Type        string    `json:"type"`
	Buckets     []float64 `json:"buck,omitempty"`
	RoundsAfter []float64 `json:"roundsAfter,omitempty"`
	Times       []float64 `json:"times,omitempty"`
}

type SysEntry struct {
	File     string  `json:"file"`
	Type     string  `json:"type"`
	SysTime  float64 `json:"systime"`
	UserTime float64 `json:"usertime"`
}

// General interface to have each app have its own statistics displayed
type Stats interface {
	// Tell the stats to write on this specific writer
	// user responsability to close it after if needed
	// but it could be a net.Conn or whatever !
	// Call this before ServerCSV* or ClientCSV*
	WriteTo(w io.Writer)
	ServerCSVHeader() error
	ServerCSV() error
	// had to keep client stats tight together because
	// of the coll_stamp server's stats that needs the rate from the client
	ClientCSV() error
	ClientCSVHeader() error

	// incoporate the Entry into theses stats
	// i is the "Index" of this entry (i.e. # times we have added entries)
	AddEntry(e Entry) error
	// Valid tells is the stats received some real data and is not
	// empty or full of garbage
	Valid() bool

	// Average will let you average a bunch of stats from a already existing
	Average(stats ...Stats) (Stats, error)
}

// statistics about the shamir_sign app
type ShamirStats struct {
	// the writer to write the stats
	Writer io.Writer
	// number of hosts
	NHosts int

	// times for the rounds
	round StreamStats
	// times for the setup
	setup StreamStats

	SysTime  float64
	UserTime float64
}

func (s *ShamirStats) WriteTo(w io.Writer) {
	s.Writer = w
}

// Return the CSV header of theses stats.
// Could be implemented using reflection for automatic detection later .. ?
func (s *ShamirStats) ServerCSVHeader() error {
	_, err := fmt.Fprintf(s.Writer, "Hosts, %s, %s, user, system\n", s.round.Header("round_"), s.setup.Header("setup_"))
	return err
}

func (s *ShamirStats) ServerCSV() error {
	_, err := fmt.Fprintf(s.Writer, "%d, %s, %s, %f, %f\n",
		s.NHosts,
		s.round.String(),
		s.setup.String(),
		s.UserTime/1e9,
		s.SysTime/1e9)
	return err
}

func (s *ShamirStats) ClientCSVHeader() error {
	return nil
}
func (s *ShamirStats) ClientCSV() error {
	return nil
}

// Add an entry to the global stats
func (s *ShamirStats) AddEntry(e Entry) error {
	switch t := e.(type) {
	// the entry is a ShamirEntry !
	case ShamirEntry:
		st := e.(ShamirEntry)
		// is it about the Round , or the setup
		if st.Type == ShamirRoundType {
			s.round.Update(st.Time)
		} else if st.Type == ShamirSetupType {
			s.setup.Update(st.Time)
		} else {
			dbg.Fatal("Received unknown shamir entry : ", st.Type)
		}
	case SysEntry:
		st := e.(SysEntry)
		s.SysTime = st.SysTime
		s.UserTime = st.UserTime
	default:
		dbg.Fatal("Received unknown entry type : ", t)
	}
	return nil
}

// basic check to see if we got somme real data
func (s *ShamirStats) Valid() bool {
	return s.round.Avg() > 0.0 && s.setup.Avg() > 0.0
}

// Average all these stats
func (s *ShamirStats) Average(stats ...Stats) (Stats, error) {
	if len(stats) < 1 {
		return s, nil
	}
	s.SysTime = 0
	s.UserTime = 0
	stset := make([]StreamStats, len(stats))
	stround := make([]StreamStats, len(stats))
	for i, _ := range stats {
		ss, ok := stats[i].(*ShamirStats)
		if !ok {
			return nil, errors.New("Average() received a non-shamir stats ")
		}
		stset[i] = ss.setup
		stround[i] = ss.round
		s.SysTime += ss.SysTime
		s.UserTime += ss.UserTime
	}
	s.setup = StreamStatsAverage(stset...)
	s.round = StreamStatsAverage(stround...)
	s.SysTime /= float64(len(stats))
	s.UserTime /= float64(len(stats))
	return s, nil
}

// Collective signing stats
type CollStats struct {
	// number of hosts
	NHosts int
	// Writer where to write the data
	Writer io.Writer

	Depth int

	BF    int
	round StreamStats

	SysTime  float64
	UserTime float64

	Rate  float64
	Times []float64
}

func (c *CollStats) Valid() bool {
	return c.round.Avg() > 0.0 && c.Rate > 0.0
}

// Simple setter for the writer
func (c *CollStats) WriteTo(w io.Writer) {
	c.Writer = w
}

// Write the CSV Header for stats about collective signing
func (s *CollStats) ServerCSVHeader() error {
	_, err := fmt.Fprintf(s.Writer, "hosts, depth, bf, %s, rate, systime, usertime\n", s.round.Header(""))
	return err
}
func (s *CollStats) ServerCSV() error {
	_, err := fmt.Fprintf(s.Writer, "%d, %d, %d, %s, %f, %f, %f\n",
		s.NHosts,
		s.Depth,
		s.BF,
		s.round.String(),
		s.Rate,
		s.SysTime/1e9,
		s.UserTime/1e9)
	return err
}

func (s *CollStats) ClientCSVHeader() error {
	_, err := fmt.Fprintf(s.Writer, "client_times\n")
	return err
}
func (s *CollStats) ClientCSV() error {
	for _, t := range s.Times {
		_, err := fmt.Fprintf(s.Writer, strconv.FormatFloat(t/1e9, 'f', 15, 64)+"\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// Add another entry that updates the stats
func (s *CollStats) AddEntry(e Entry) error {
	switch e.(type) {
	case CollServerEntry:
		cse := e.(CollServerEntry)
		s.round.Update(cse.Time)
	case CollClientEntry:
		// what do I want to keep out of the Client Message States
		// cms.Buckets stores how many were processed at time T
		// cms.RoundsAfter stores how many rounds delayed it was
		//
		// get the average delay (roundsAfter), max and min
		// get the total number of messages timestamped
		// get the average number of messages timestamped per second?
		// get the observed rate of processed messages
		// avg is how many messages per second, we want how many milliseconds between messages
		cce := e.(CollClientEntry)
		avg, _, _, _ := ArrStats(cce.Buckets)
		observed := avg / 1000
		observed = 1 / observed
		s.Rate = observed
		s.Times = cce.Times
	case SysEntry:
		se := e.(SysEntry)
		s.SysTime = se.SysTime
		s.UserTime = se.UserTime
	default:
		dbg.Fatal("AddEntry did not receive any Coll*Entry.")
	}
	return nil
}

// Average a collection of Stats that better be CollStats !
func (s *CollStats) Average(stats ...Stats) (Stats, error) {
	if len(stats) == 0 {
		return s, errors.New("No stats given to average on CollStats")
	}
	s.Times = make([]float64, len(s.Times))
	st := make([]StreamStats, 0, len(stats))
	s.SysTime = 0
	s.UserTime = 0
	s.Rate = 0
	for _, b := range stats {
		a, ok := b.(*CollStats)
		if !ok {
			return nil, errors.New("Average() did not receive a CollStats struct")
		}
		st = append(st, a.round)
		s.SysTime += a.SysTime
		s.UserTime += a.UserTime
		s.Rate += a.Rate
		s.Times = append(s.Times, a.Times...)
	}
	s.round = StreamStatsAverage(st...)
	l := float64(len(stats))
	s.SysTime /= l
	s.UserTime /= l
	s.Rate /= l
	return s, nil
}

// Wrapper function to average multiple stats
// they must be of the same type  !
func AverageStats(stats ...Stats) (Stats, error) {
	if len(stats) == 0 {
		return nil, errors.New("No stats given to average")
	}
	return stats[0].Average(stats...)
}

// helper function to get the right Stats depending on the test
func GetStat(t T) Stats {
	switch t.app {
	case ShamirSign:
		return NewShamirStats(t)
	case CollSign, CollStamp:
		return NewCollStats(t)
	}
	return nil
}

func NewShamirStats(t T) *ShamirStats {
	return &ShamirStats{NHosts: t.nmachs * t.hpn}
}

func NewCollStats(t T) *CollStats {
	depth := math.Log(float64(t.nmachs*t.hpn)*float64(t.bf-1) + 1)
	depth /= math.Log(float64(t.bf))
	depth = math.Ceil(depth)
	depth -= 1
	return &CollStats{
		BF:     t.bf,
		NHosts: t.nmachs * t.hpn,
		Depth:  int(depth),
	}
}

type ExpVar struct {
	Cmdline  []string         `json:"cmdline"`
	Memstats runtime.MemStats `json:"memstats"`
}

func Memstats(server string) (*ExpVar, error) {
	url := "localhost:8081/d/" + server + "/debug/vars"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var evar ExpVar
	err = json.Unmarshal(b, &evar)
	if err != nil {
		log.Println("failed to unmarshal expvar:", string(b))
		return nil, err
	}
	return &evar, nil
}

func MonitorMemStats(server string, poll int, done chan struct{}, stats *[]*ExpVar) {
	go func() {
		ticker := time.NewTicker(time.Duration(poll) * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				evar, err := Memstats(server)
				if err != nil {
					continue
				}
				*stats = append(*stats, evar)
			case <-done:
				return
			}
		}
	}()
}

func ArrStats(stream []float64) (avg float64, min float64, max float64, stddev float64) {
	// truncate trailing 0s
	i := len(stream) - 1
	for ; i >= 0; i-- {
		if math.Abs(stream[i]) > 0.01 {
			break
		}
	}
	stream = stream[:i+1]

	k := float64(1)
	first := true
	var M, S float64
	for _, e := range stream {
		if first {
			first = false
			min = e
			max = e
		}
		if e < min {
			min = e
		} else if max < e {
			max = e
		}
		avg = ((avg * (k - 1)) + e) / k
		var tM = M
		M += (e - tM) / k
		S += (e - tM) * (e - M)
		k++
		stddev = math.Sqrt(S / (k - 1))
	}
	return avg, min, max, stddev
}
