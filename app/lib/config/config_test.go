package config

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/stretchr/testify/assert"
)

var b bytes.Buffer
var o *bufio.Writer

func TestMain(m *testing.M) {
	o = bufio.NewWriter(&b)
	out = o
	log.MainTest(m)
}

var serverGroup string = `Description = "Default Dedis Cosi servers"

[[servers]]
Address = "tcp://5.135.161.91:2000"
Public = "lLglU3nhHfUWe4p647hffn618TiUq+6FvTGzJw8eTGU="
Description = "Nikkolasg's server: spreading the love of signing"

[[servers]]
Address = "tcp://185.26.156.40:61117"
Public = "apIWOKSt6JcOvNnjcVcPCNcaJJh/kPEjkbn2xSW+W+Q="
Description = "Ismail's server"`

func TestReadGroupDescToml(t *testing.T) {
	group, err := ReadGroupDescToml(strings.NewReader(serverGroup))
	log.ErrFatal(err)

	if len(group.Roster.List) != 2 {
		t.Fatal("Should have 2 ServerIdentities")
	}
	nikkoAddr := group.Roster.List[0].Address
	if !nikkoAddr.Valid() || nikkoAddr != network.NewTCPAddress("5.135.161.91:2000") {
		t.Fatal("Address not valid " + group.Roster.List[0].Address.String())
	}
	if len(group.description) != 2 {
		t.Fatal("Should have 2 descriptions")
	}
	if group.description[group.Roster.List[1]] != "Ismail's server" {
		t.Fatal("This should be Ismail's server")
	}
}

func TestInput(t *testing.T) {
	setInput("Y")
	assert.Equal(t, "Y", Input("def", "Question"))
	assert.Equal(t, "Question [def]: ", getOutput())
	setInput("")
	assert.Equal(t, "def", Input("def", "Question"))
	setInput("1\n2")
	assert.Equal(t, "1", Input("", "Question1"))
	assert.Equal(t, "2", Input("1", "Question2"))
}

func TestInputYN(t *testing.T) {
	setInput("")
	assert.True(t, InputYN(true))
	setInput("")
	assert.False(t, InputYN(false, "Are you sure?"))
	assert.Equal(t, "Are you sure? [Ny]: ", getOutput())
	setInput("")
	assert.True(t, InputYN(true, "Are you sure?"))
	assert.Equal(t, "Are you sure? [Yn]: ", getOutput(), "one")
}

func setInput(s string) {
	// Flush output
	getOutput()
	in = bufio.NewReader(bytes.NewReader([]byte(s + "\n")))
}

func getOutput() string {
	o.Flush()
	ret := b.String()
	b.Reset()
	return ret
}
