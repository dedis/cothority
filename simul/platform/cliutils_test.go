package platform

import (
	"io/ioutil"
	"testing"

	"crypto/rand"

	"bytes"

	"os"

	"path"

	"github.com/dedis/cothority/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestCopy(t *testing.T) {
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

func TestCopy2(t *testing.T) {
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
