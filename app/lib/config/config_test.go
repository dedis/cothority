package config

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"path"
	"strings"
	"testing"

	"io/ioutil"

	"os"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCopy(t *testing.T) {
	tmp, err := ioutil.TempFile("", "copy")
	log.ErrFatal(err)
	_, err = tmp.Write([]byte{3, 1, 4, 5, 9, 2, 6})
	log.ErrFatal(err)
	log.ErrFatal(tmp.Close())
	nsrc := tmp.Name()
	ndst := nsrc + "1"
	log.ErrFatal(Copy(ndst, nsrc))
	stat, err := os.Stat(ndst)
	log.ErrFatal(err)
	require.Equal(t, int64(7), stat.Size())
	log.ErrFatal(os.Remove(nsrc))
	log.ErrFatal(os.Remove(ndst))
}

func TestCopy2(t *testing.T) {
	// Create random-file with length (nearly) double of the blocksize.
	tmpfile, err := ioutil.TempFile("", "test")
	defer func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}()
	log.ErrFatal(err)
	var rnd = make([]byte, (1<<17)-2)
	n, err := rand.Read(rnd)
	log.ErrFatal(err)
	nWrite, err := tmpfile.Write(rnd)
	log.ErrFatal(err)
	if nWrite != n {
		t.Fatal("Wrote", nWrite, "out of", n, "bytes")
	}
	tmpfile.Close()

	// Copy to a second tmp-file
	tmpfile2, err := ioutil.TempFile("", "test")
	log.ErrFatal(tmpfile2.Close())
	defer os.Remove(tmpfile2.Name())
	log.ErrFatal(Copy(tmpfile2.Name(), tmpfile.Name()))

	// Compare the two files
	oStat, err := os.Stat(tmpfile.Name())
	log.ErrFatal(err)
	cStat, err := os.Stat(tmpfile2.Name())
	log.ErrFatal(err)

	if oStat.Mode() != cStat.Mode() {
		t.Fatal("Modes are not equal.")
	}
	if oStat.Size() != cStat.Size() {
		t.Fatal("Sizes are not equal:", oStat.Size(), cStat.Size())
	}

	copySlice, err := ioutil.ReadFile(tmpfile2.Name())
	log.ErrFatal(err)
	if bytes.Compare(rnd, copySlice) != 0 {
		t.Fatal("File-contents are not the same.")
	}
}

func TestCopy3(t *testing.T) {
	// Test copying to a directory
	tmpfile, err := ioutil.TempFile("", "test")
	log.ErrFatal(err)
	defer func() {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
	}()
	tmpdir, err := ioutil.TempDir("", "copydir")
	log.ErrFatal(err)
	defer os.RemoveAll(tmpdir)

	log.ErrFatal(Copy(tmpdir, tmpfile.Name()))
	copy, err := os.Open(path.Join(tmpdir, path.Base(tmpfile.Name())))
	log.ErrFatal(err)
	log.ErrFatal(copy.Close())
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
