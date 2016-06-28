package sda

//Status is the status server
type Status map[string]string

//StatusReporter is the interface that all things that want to return a status will implement
type StatusReporter interface {
	GetStatus() Status
}

//statusReporter struct is a struct
type statusReporterStruct struct {
	statusReporters map[string]StatusReporter
}

//newStatusReporterStruct creats the struct
func newStatusReporterStruct() *statusReporterStruct {
	return &statusReporterStruct{
		statusReporters: make(map[string]StatusReporter),
	}
}

//RegisterStatusReporter registers a status reporter within struct
func (s *statusReporterStruct) RegisterStatusReporter(name string, sr StatusReporter) {
	s.statusReporters[name] = sr

}

//ReportStatus gets the status of all StatusReporters within the Registry and puts them in a map
func (s *statusReporterStruct) ReportStatus() map[string]Status {
	m := make(map[string]Status)
	for key, val := range s.statusReporters {
		m[key] = val.GetStatus()
	}
	return m
}
