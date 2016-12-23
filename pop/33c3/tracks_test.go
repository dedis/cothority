package main

import (
	"io/ioutil"
	"testing"

	"os"

	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	schedule33c3 = "schedule_format.json"
	prgid        = 7911
)

func TestDatabase__VotesSave(t *testing.T) {
	db := newDatabase()
	db.load(schedule33c3)

	db.DB[prgid].Votes["a"] = true
	tmpfile, err := ioutil.TempFile("", "db")
	log.ErrFatal(err)
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())
	log.ErrFatal(db.VotesSave(tmpfile.Name()))
}

func TestDatabase__VotesLoad(t *testing.T) {
	db := newDatabase()
	db.load(schedule33c3)
	db.DB[prgid].Votes["a"] = true
	tmpfile, err := ioutil.TempFile("", "db")
	log.ErrFatal(err)
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())
	log.ErrFatal(db.VotesSave(tmpfile.Name()))

	db2 := newDatabase()
	log.ErrFatal(db2.VotesLoad(schedule33c3, tmpfile.Name()))
	require.True(t, db2.DB[prgid].Votes["a"])
}

func TestSessionStore__Save(t *testing.T) {
	st := newSessionStore()
	st.Sessions["one"] = true
	st.Nonces["two"] = true

	tmpfile, err := ioutil.TempFile("", "st")
	log.ErrFatal(err)
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())
	st.Save(tmpfile.Name())

	st2 := newSessionStore()
	st2.Load(tmpfile.Name())
	assert.Equal(t, st, st2)
}
