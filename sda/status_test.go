package sda

import (
	"testing"

	"strconv"

	"github.com/stretchr/testify/assert"
)

func TestSRStruct(t *testing.T) {
	srs := newStatusReporterStruct()
	assert.NotNil(t, srs)
	dtr := &dummyTestReporter{5}
	srs.RegisterStatusReporter("Dummy", dtr)
	assert.Equal(t, srs.ReportStatus()["Dummy"]["Connections"], "5")
	dtr.Status = 10
	assert.Equal(t, srs.ReportStatus()["Dummy"]["Connections"], "10")
}

type dummyTestReporter struct {
	Status int
}

func (d *dummyTestReporter) GetStatus() Status {
	return Status{
		"Connections": strconv.Itoa(d.Status),
	}
}
