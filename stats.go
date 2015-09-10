package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

/*
{
	"eapp":"time",
	"ehost":"10.255.0.13:2000",
	"elevel":"info",
	"emsg":"root round",
	"etime":"2015-02-27T09:50:45-08:00",
	"file":"server.go:195",
	"round":59,
	"time":893709029,
	"type":"root_round"
}
*/
type StatsEntry struct {
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

type SysStats struct {
	File     string  `json:"file"`
	Type     string  `json:"type"`
	SysTime  float64 `json:"systime"`
	UserTime float64 `json:"usertime"`
}

type ClientMsgStats struct {
	File        string    `json:"file"`
	Type        string    `json:"type"`
	Buckets     []float64 `json:"buck,omitempty"`
	RoundsAfter []float64 `json:"roundsAfter,omitempty"`
	Times       []float64 `json:"times,omitempty"`
}

type RunStats struct {
	NHosts int
	Depth  int

	BF int

	MinTime float64
	MaxTime float64
	AvgTime float64
	StdDev  float64

	SysTime  float64
	UserTime float64

	Rate  float64
	Times []float64
}

func (s RunStats) CSVHeader() []byte {
	var buf bytes.Buffer
	buf.WriteString("hosts, depth, bf, min, max, avg, stddev, systime, usertime, rate\n")
	return buf.Bytes()
}
func (s RunStats) CSV() []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d, %d, %d, %f, %f, %f, %f, %f, %f, %f\n",
		s.NHosts,
		s.Depth,
		s.BF,
		s.MinTime/1e9,
		s.MaxTime/1e9,
		s.AvgTime/1e9,
		s.StdDev/1e9,
		s.SysTime/1e9,
		s.UserTime/1e9,
		s.Rate)
	return buf.Bytes()
}

func (s RunStats) TimesCSV() []byte {
	times := bytes.Buffer{}
	times.WriteString("client_times\n")
	for _, t := range s.Times {
		times.WriteString(strconv.FormatFloat(t/1e9, 'f', 15, 64))
		times.WriteRune('\n')
	}
	return times.Bytes()
}

func RunStatsAvg(rs []RunStats) RunStats {
	if len(rs) == 0 {
		return RunStats{}
	}
	r := RunStats{}
	r.NHosts = rs[0].NHosts
	r.Depth = rs[0].Depth
	r.BF = rs[0].BF
	r.Times = make([]float64, len(rs[0].Times))

	for _, a := range rs {
		r.MinTime += a.MinTime
		r.MaxTime += a.MaxTime
		r.AvgTime += a.AvgTime
		r.StdDev += a.StdDev
		r.SysTime += a.SysTime
		r.UserTime += a.UserTime
		r.Rate += a.Rate
		r.Times = append(r.Times, a.Times...)
	}
	l := float64(len(rs))
	r.MinTime /= l
	r.MaxTime /= l
	r.AvgTime /= l
	r.StdDev /= l
	r.SysTime /= l
	r.UserTime /= l
	r.Rate /= l
	return r
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
