package monitor

import (
	"fmt"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/montanaflynn/stats"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type DataFilter struct {
	// percentiles maps the measurements name to the percentile we need to take
	// to filter thoses measuremements with the percentile
	percentiles map[string]float64
}

// NewDataFilter returns a new data filter initialized with the rights values
// taken out from the run config. If absent, will take defaults values.
// Keys expected are:
// discard_measurementname = perc => will take the lower and upper percentile =
// perc
// discard_measurementname = lower,upper => will take different percentiles
func NewDataFilter(config map[string]string) DataFilter {
	df := DataFilter{
		percentiles: make(map[string]float64),
	}
	reg, err := regexp.Compile("filter_(\\w+)")
	if err != nil {
		dbg.Lvl1("DataFilter: Error compiling regexp:", err)
		return df
	}
	// analyse the each entry
	for k, v := range config {
		if measure := reg.FindString(k); measure == "" {
			continue
		} else {
			// this value must be filtered by how many ?
			perc, err := strconv.ParseFloat(v, 64)
			if err != nil {
				dbg.Lvl1("DataFilter: Cannot parse value for filter measure:", measure)
				continue
			}
			measure = strings.Replace(measure, "filter_", "", -1)
			df.percentiles[measure] = perc
		}
	}
	dbg.Lvl3("Filtering:", df.percentiles)
	return df
}

// Filter out a serie of values
func (df *DataFilter) Filter(measure string, values []float64) []float64 {
	// do we have a filter for this measure ?
	if _, ok := df.percentiles[measure]; !ok {
		return values
	}
	// Compute the percentile value
	max, err := stats.PercentileNearestRank(values, df.percentiles[measure])
	if err != nil {
		dbg.Lvl2("Monitor: Error filtering data:", err)
		return values
	}

	// Find the index from where to filter
	maxIndex := -1
	for i, v := range values {
		if v > max {
			maxIndex = i
		}
	}
	// check if we foud something to filter out
	if maxIndex == -1 {
		dbg.Lvl3("Filtering: nothing to filter for", measure)
		return values
	}
	// return the values below the percentile
	dbg.Lvl3("Filtering: filters out ", measure, " :", maxIndex, " /", len(values))
	return values[:maxIndex]
}

////////////////////// HELPERS FUNCTIONS / STRUCT /////////////////
// Value is used to compute the statistics
// it reprensent the time to an action (setup, shamir round, coll round etc)
// use it to compute streaming mean + dev
type Value struct {
	min float64
	max float64

	n    int
	oldM float64
	newM float64
	oldS float64
	newS float64
	dev  float64

	// Store where are kept the values
	store []float64
}

func NewValue() *Value {
	return &Value{store: make([]float64, 0)}
}

// Store takes this new time and stores it for later analysis
// Since we might want to do percentile sorting, we need to have all the values
// For the moment, we do a simple store of the value, but note that some
// streaming percentile algorithm exists in case the number of messages is
// growing to big.
func (t *Value) Store(newTime float64) {
	t.store = append(t.store, newTime)
}

// Aggregate will aggregate all values stored in the store's Value.
// It is kept as a streaming average / dev processus fr the moment (not the most
// optimized).
// streaming dev algo taken from http://www.johndcook.com/blog/standard_deviation/
func (t *Value) Aggregate(measure string, df DataFilter) {
	t.store = df.Filter(measure, t.store)
	for _, newTime := range t.store {
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
}

// Average will set the current Value to the average of all Value
func AverageValue(st ...*Value) *Value {
	var t Value
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
	return &t
}

func (t *Value) Min() float64 {
	return t.min
}
func (t *Value) Max() float64 {
	return t.max
}

// return the number of value added
func (t *Value) NumValue() int {
	return t.n
}

func (t *Value) Avg() float64 {
	return t.newM
}

func (t *Value) Dev() float64 {
	return t.dev
}

func (t *Value) Header(prefix string) string {
	return fmt.Sprintf("%s_min, %s_max, %s_avg, %s_dev", prefix, prefix, prefix, prefix)
}
func (t *Value) String() string {
	return fmt.Sprintf("%f, %f, %f, %f", t.Min(), t.Max(), t.Avg(), t.Dev())
}

// Measurement represents the precise measurement of a specific thing to measure
// example: I want to measure the time it takes to verify a signature, the
// measurement "verify" will hold a wallclock Value, cpu_user Value, cpu_system
// Value
type Measurement struct {
	Name   string
	Wall   *Value
	User   *Value
	System *Value
	Filter DataFilter
}

func NewMeasurement(name string, df DataFilter) Measurement {
	return Measurement{
		Name:   name,
		Wall:   NewValue(),
		User:   NewValue(),
		System: NewValue(),
		Filter: df,
	}
}

// WriteHeader will write the header to the specified writer
func (m *Measurement) WriteHeader(w io.Writer) {
	fmt.Fprintf(w, "%s, %s, %s", m.Wall.Header(m.Name+"_wall"),
		m.User.Header(m.Name+"_user"), m.System.Header(m.Name+"_system"))
}

// WriteValues will write a new entry for this entry in the writer
// First compute the values then write to writer
func (m *Measurement) WriteValues(w io.Writer) {
	fmt.Fprintf(w, "%s, %s, %s", m.Wall.String(), m.User.String(), m.System.String())
}

// Update takes a measure received from the network and update the wall system
// and user values
func (m *Measurement) Update(measure Measure) {
	dbg.Lvl2("Got measurement for", m.Name, measure.WallTime, measure.CPUTimeUser, measure.CPUTimeSys)
	m.Wall.Store(measure.WallTime)
	m.User.Store(measure.CPUTimeUser)
	m.System.Store(measure.CPUTimeSys)
}

// AverageMeasurements takes an slice of measurements and make the average
// between them. i.e. it takes the average of the Wall value from each
// measurements, etc.
func AverageMeasurements(measurements []Measurement) Measurement {
	m := NewMeasurement(measurements[0].Name, measurements[0].Filter)
	walls := make([]*Value, len(measurements))
	users := make([]*Value, len(measurements))
	systems := make([]*Value, len(measurements))
	for i, m := range measurements {
		m.Wall.Aggregate(m.Name, m.Filter)
		m.User.Aggregate(m.Name, m.Filter)
		m.System.Aggregate(m.Name, m.Filter)
		walls[i] = m.Wall
		users[i] = m.User
		systems[i] = m.System
	}
	m.Wall = AverageValue(walls...)
	m.User = AverageValue(users...)
	m.System = AverageValue(systems...)
	return m
}

func (m *Measurement) String() string {
	return fmt.Sprintf("{Measurement %s : wall = %v, system = %v, user = %v}", m.Name, m.Wall, m.User, m.System)
}

// Stats holds the different measurements done
type Stats struct {
	// How many peers do we have
	Peers int
	// How many peers per machine do we use
	PPM int // PeerPerMachine
	// How many machines do we have
	Machines int
	// How many peers are ready
	Ready int

	// Additionals fields that may appears in the resulting CSV
	// The additionals fields are created when creating the stats out of a
	// running config. It will try to read some known fields such as "depth" or
	// "bf" (branching factor) and add then to its struct
	Additionals map[string]int

	// The measures we have and the keys ordered
	measures map[string]*Measurement
	keys     []string

	// The filter used to filter out abberant data
	filter DataFilter
}

// ExtraFields in a RunConfig argument that we may want to parse if present
var extraFields = [...]string{"bf", "rate", "stampratio"}

// DefaultMeasurements are the default measurements we want to do anyway
// For now these will be the fields that will appear in the output csv file
var DefaultMeasurements = [...]string{"setup", "round", "calc", "verify", "aggregate"}

// Return a NewStats with some fields extracted from the platform run config
// It enforces the default set of measure to do.
func NewStats(rc map[string]string) *Stats {
	s := new(Stats).Init()
	s.readRunConfig(rc)
	for _, d := range DefaultMeasurements {
		s.AddMeasurements(d)
	}
	return s
}

// Read a config file and fills up some fields for Stats struct
func (s *Stats) readRunConfig(rc map[string]string) {
	if machs, err := strconv.Atoi(rc["machines"]); err != nil {
		dbg.Fatal("Can not create stats from RunConfig with no machines")
	} else {
		s.Machines = machs
	}
	if ppm, err := strconv.Atoi(rc["ppm"]); err != nil {
		dbg.Fatal("Can not create stats from RunConfig with no ppm")
	} else {
		s.PPM = ppm
	}
	s.Peers = s.Machines * s.PPM
	// Add some extra fields if recognized
	for _, f := range extraFields {
		if ef, err := strconv.Atoi(rc[f]); err != nil {
			continue
		} else {
			s.Additionals[f] = ef
		}
	}
	// let the filter figure out itself what it is supposed to be doing
	s.filter = NewDataFilter(rc)
}

func (s *Stats) Init() *Stats {
	s.measures = make(map[string]*Measurement)
	s.keys = make([]string, 0)
	s.Additionals = make(map[string]int)
	return s
}

// AddMeasurement is used to notify which measures we want to record
// Each name given will be outputted in the CSV file
// If stats receive a Measure which is not known, it will be discarded.
// THIS IS THE DEFAULT BEHAVIOR FOR NOW.
func (s *Stats) AddMeasurements(measurements ...string) {
	for _, name := range measurements {
		if _, ok := s.measures[name]; !ok {
			m := NewMeasurement(name, s.filter)
			s.measures[name] = &m
			s.keys = append(s.keys, name)
		}
	}
}

// WriteHeader will write the header to the writer
func (s *Stats) WriteHeader(w io.Writer) {
	// write basic info
	fmt.Fprintf(w, "Peers, ppm, machines")
	// write additionals fields
	for _, k := range extraFields {
		if _, ok := s.Additionals[k]; ok {
			fmt.Fprintf(w, ", %s", k)
		}
	}
	// Write the values header
	for _, k := range s.keys {
		fmt.Fprintf(w, ", ")
		m := s.measures[k]
		m.WriteHeader(w)
	}
	fmt.Fprintf(w, "\n")
}

// WriteValues will write the values to the specified writer
func (s *Stats) WriteValues(w io.Writer) {
	// write basic info
	fmt.Fprintf(w, "%d, %d, %d", s.Peers, s.PPM, s.Machines)
	// write additionals fields
	for _, k := range extraFields {
		v, ok := s.Additionals[k]
		if ok {
			fmt.Fprintf(w, ", %d", v)
		}
	}
	// write the values
	for _, k := range s.keys {
		fmt.Fprintf(w, ", ")
		m := s.measures[k]
		m.WriteValues(w)
	}
	fmt.Fprintf(w, "\n")

}

// AverageStats will make an average of the given stats
func AverageStats(stats []Stats) Stats {
	if len(stats) < 1 {
		return Stats{}
	}
	s := new(Stats).Init()
	s.Machines = stats[0].Machines
	s.PPM = stats[0].PPM
	s.Peers = stats[0].Peers
	s.Additionals = stats[0].Additionals
	// Collect measurements name
	for _, stat := range stats {
		s.AddMeasurements(stat.keys...)
	}
	// Average
	for _, k := range s.keys {
		// Collect measurements for a given key
		measurements := make([]Measurement, len(stats))
		for i, stat := range stats {
			sub, ok := stat.measures[k]
			if !ok {
				continue
			}
			measurements[i] = *sub
		}
		// make the average
		avg := AverageMeasurements(measurements)
		s.measures[k] = &avg
	}
	return *s
}

// Update will update the Stats with this given measure
func (s *Stats) Update(m Measure) {
	meas, ok := s.measures[m.Name]
	if !ok {
		dbg.Lvl2("Stats Update received unknown type of measure : ", m.Name)
		return
	}
	meas.Update(m)
}

func (s *Stats) String() string {
	var str string
	for _, v := range s.measures {
		str += fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("{Stats: Peers %d, Measures : %s}", s.Peers, str)
}
