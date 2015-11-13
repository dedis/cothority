package monitor

import (
	"flag"
	"fmt"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"io"
	"math"
	"strconv"
	"strings"
)

// How many measures do I discard before aggregating the statistics
type Discards struct {
	measures map[string]bool
}

// Holds the value to discard
var discards Discards

// Discards must implement the Var interface to be read by flag
// This way it allows to specify multiple measure to discard with a separated
// comma list.
func (d *Discards) String() string {
	var arr []string
	for name, _ := range d.measures {
		arr = append(arr, name)
	}
	return strings.Join(arr, ",")
}
func (d *Discards) Set(s string) error {
	arr := strings.Split(s, ",")
	d.measures = make(map[string]bool, len(arr))
	for _, meas := range arr {
		d.measures[meas] = true
	}
	return nil
}

// Reset sets every flags to true
func (d *Discards) Reset() {
	for k := range d.measures {
		d.measures[k] = true
	}
}

// Does this measure is contained in the list of discards
// if it is, look if we already discarded it or not
// Think of discardss like a middleware passing or not the value downstream
func (d *Discards) Update(newMeasure Measure, reference *Measurement) {
	for name, disc := range d.measures {
		// we must discard it and we havent seen it yet
		if name == newMeasure.Name && disc {
			d.measures[name] = !disc
			dbg.Lvl3("Monitor: discarding measure", name)
			return
		}
	}
	reference.Update(newMeasure)
}
func init() {
	discards.Set("round,verify")
	flag.Var(&discards, "discard", "Measures where we want to discard the first round ( can specify a list m1,m2,m3 ...)")
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
}

// Update will update the time struct with the min / max  change
// + compute new avg + new dev
// k is the number of times we've added something ("index" of the update)
// needed to compute the avg + dev
// streaming dev algo taken from http://www.johndcook.com/blog/standard_deviation/
func (t *Value) Update(newTime float64) {
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

// Average will set the current Value to the average of all Value
func AverageValue(st ...Value) Value {
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
	return t
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
	Wall   Value
	User   Value
	System Value
}

// WriteHeader will write the header to the specified writer
func (m *Measurement) WriteHeader(w io.Writer) {
	fmt.Fprintf(w, "%s, %s, %s", m.Wall.Header(m.Name+"_wall"),
		m.User.Header(m.Name+"_user"), m.System.Header(m.Name+"_system"))
}

// WriteValues will write a new entry for this entry in the writer
func (m *Measurement) WriteValues(w io.Writer) {
	fmt.Fprintf(w, "%s, %s, %s", m.Wall.String(), m.User.String(), m.System.String())
}

// Update takes a measure received from the network and update the wall system
// and user values
func (m *Measurement) Update(measure Measure) {
	m.Wall.Update(measure.WallTime)
	m.User.Update(measure.CPUTimeUser)
	m.System.Update(measure.CPUTimeSys)
}

// AverageMeasurements takes an slice of measurements and make the average
// between them. i.e. it takes the average of the Wall value from each
// measurements, etc.
func AverageMeasurements(measurements []Measurement) Measurement {
	m := Measurement{Name: measurements[0].Name}
	walls := make([]Value, len(measurements))
	users := make([]Value, len(measurements))
	systems := make([]Value, len(measurements))
	for i, m := range measurements {
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
	// Additionals fields that may appears in the resulting CSV
	// The additionals fields are created when creating the stats out of a
	// running config. It will try to read some known fields such as "depth" or
	// "bf" (branching factor) and add then to its struct
	Additionals map[string]int

	// The measures we have and the keys ordered
	measures map[string]*Measurement
	keys     []string
}

// ExtraFIelds in a RunConfig argument that we may want to parse if present
var extraFields = [...]string{"bf"}

// DefaultMeasurements are the default measurements we want to do anyway
// For now these will be the fields that will appear in the output csv file
var DefaultMeasurements = [...]string{"setup", "round", "calc", "verify"}

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
			s.measures[name] = &Measurement{Name: name}
			s.keys = append(s.keys, name)
		}
	}
}

// WriteHeader will write the header to the writer
func (s *Stats) WriteHeader(w io.Writer) {
	// write basic info
	fmt.Fprintf(w, "Peers, ppm, machines")
	// write additionals fields
	for k, _ := range s.Additionals {
		fmt.Fprintf(w, ", %s", k)
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
	for _, v := range s.Additionals {
		fmt.Fprintf(w, ", %d", v)
	}
	// write the values
	for _, k := range s.keys {
		fmt.Fprintf(w, ", ")
		m := s.measures[k]
		m.WriteValues(w)
	}
	fmt.Fprintf(w, "\n")

	// Reset the discards
	discards.Reset()
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
	discards.Update(m, meas)
}

func (s *Stats) String() string {
	var str string
	for _, v := range s.measures {
		str += fmt.Sprintf("%v", v)
	}
	return fmt.Sprintf("{Stats: Peers %d, Measures : %s}", s.Peers, str)
}
