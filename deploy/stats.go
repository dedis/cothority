package deploy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dedis/cothority/lib/debug_lvl"
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
	n += 1
	if t.min > newTime {
		t.min = newTime
	}
	if t.max < newTime {
		t.max = newTime
	}

	if n == 1 {
		t.oldM = newTime
		t.newM = newTime
		t.oldS = 0.0
	} else {
		t.newM = oldM + (newTime-oldM)/n
		t.newS = oldS + (newTime-oldM)*(newTime-newM)
		t.oldM = newM
		t.oldS = newS
	}
	t.dev = math.Sqrt(newS / (n - 1))

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
	l := len(st)
	t.min /= l
	t.max /= l
	t.newM /= l
	t.dev /= l
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
	if t.n > 0 {
		return t.newM
	}
	return 0.0
}

func (t *StreamStats) Dev() float64 {
	if t.n > 1 {
		return t.dev
	}
	return 0.0
}

func (t *StreamStats) String() string {
	return fmt.Sprintf("%f, %f, %f, %f", t.Min()/1e9, t.Max()/1e9, t.Avg()/1e9, t.Dev()/1e9)
}

////////////////////////////////////////////////////////////////

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
}

func (s *ShamirStats) WriteTo(w io.Writer) {
	s.Writer = w
}

// Return the CSV header of theses stats.
// Could be implemented using reflection for automatic detection later .. ?
func (s *ShamirStats) ServerCSVHeader() error {
	_, err := fmt.Fprintf(s.Writer, "Hosts, round_min, round_max, round_avg, setup_min, setup_max, setup_avg\n")
	return err
}

func (s *ShamirStats) ServerCSV() []byte {
	_, err := fmt.Fprintf(s.Writer, "%d, %s,%s\n",
		s.NHosts,
		round.String(),
		setup.String())
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
	switch e.(type) {
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
	default:
		dbg.Fatal("Received unknown entry type : ", e.(type))
	}
}

// Average all these stats
func ShamirStatsAverage(stats ...Stats) (StreamStats, error) {
	var s ShamirStats
	if len(stats) < 1 {
		return s
	}
	s.NHosts = stats[0].NHosts
	s.Writer = stats[0].Writer

	for _, si := range stats {
		switch si.(type) {
		case ShamirStats:
			ss := si.(ShamirStats)
			s.round.Update(ss.round)
			s.setup.Update(ss.setup)
		default:
			return s, errors.New("Average() received a stats that is not ShamirStat")
		}
	}
	return s, nil
}

// Collective signing stats
type CollStats struct {
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

// Simple setter for the writer
func (c *CollStats) WriteTo(w io.Writer) {
	c.Writer = w
}

// Write the CSV Header for stats about collective signing
func (s *CollStats) ServerCSVHeader() error {
	_, err := fmt.Fprintf(s.Writer, "hosts, depth, bf, min, max, avg, stddev, rate, systime, usertime\n")
	return err
}
func (s *CollStats) ServerCSV() error {
	_, err := fmt.Fprintf(s.Writer, "%d, %d, %d, %s, %f, %f, %f, %f, %f, %f, %f\n",
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
		obs := avg / 1000
		obs = 1 / observed
		s.Rate = observed
		s.Times = cce.Times
	case SysEntry:
		se := e.(SysEntry)
		s.Systime = se.SysTime
		s.UserTime = se.UserTime
	default:
		dbg.Fatal("AddEntry did not receive any Coll*Entry.")
	}
	return nil
}

// Average a collection of Stats that better be CollStats !
func collStatsAverage(stats ...Stats) (CollStats, error) {
	var s CollStats
	if len(rs) == 0 {
		return s, errors.New("No stats given to average on CollStats")
	}
	s.NHosts = rs[0].NHosts
	s.Depth = rs[0].Depth
	s.BF = rs[0].BF
	s.Times = make([]float64, len(stats[0].Times))

	for _, b := range rs {
		switch b.(type) {
		case CollStats:
			a := b.(CollStats)
			s.round.Average(a.round)
			s.SysTime += a.SysTime
			s.UserTime += a.UserTime
			s.Rate += a.Rate
			s.Times = append(s.Times, a.Times...)
		default:
			return s, errors.New("Average did not receive a CollStats struct")
		}
	}
	l := float64(len(stats))
	s.MinTime /= l
	s.MaxTime /= l
	s.AvgTime /= l
	s.StdDev /= l
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
	switch stats[0].(type) {
	case CollStats:
		return collStatsAverage(stats)
	case ShamirStats:
		return shamirStatsAverage(stats)
	}
}

// helper function to get the right Stats depending on the test
func GetStat(t Test) Stats {
	switch test.app {
	case ShamirSign:
		return NewShamirStats(t)
	case CollSign, CollStamp:
		return NewCollStats(t)
	}
}

func NewShamirStats(t Test) ShamirStats {
	return ShamirStats{NHosts: t.nmachs * t.hpn}
}
func NewCollStats(t Test) CollStats {
	depth := math.Log(t.nmachs*t.hpn) * t.bf
	depth /= math.Log(t.bf)
	depth -= 1
	return CollStats{
		BF:     t.bf,
		NHosts: t.nmachs * t.hpn,
		Depth:  depth,
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
