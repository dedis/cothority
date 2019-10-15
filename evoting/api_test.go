package evoting_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.dedis.ch/onet/v4"
	"go.dedis.ch/onet/v4/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.dedis.ch/cothority/v3"
	"go.dedis.ch/cothority/v3/evoting"
	_ "go.dedis.ch/cothority/v3/evoting/service"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestPing(t *testing.T) {
	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := evoting.NewClient()
	r, _ := c.Ping(roster, 0)
	assert.Equal(t, uint32(1), r.Nonce)
}

func TestLookupSciper(t *testing.T) {
	const testCard = `
BEGIN:VCARD
VERSION:2.1
FN;CHARSET=UTF-8:Martin Vetterli
N;CHARSET=UTF-8:Vetterli;Martin;;;
ADR;TYPE=WORK;CHARSET=UTF-8:EPFL PRES ; CE 3 316 (Centre Est) ; Station 1 ; CH-1015 Lausanne;Switzerland
EMAIL;TYPE=INTERNET:martin.vetterli@epfl.ch
URL:https://people.epfl.ch/martin.vetterli
TITLE;CHARSET=UTF-8:
TEL;TYPE=WORK:+41216930505 
ORG:EPFL
END:VCARD
Location: https://people.epfl.ch/cgi-bin/people/priv?id=107537&lang=
`

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "id=000000" {
			// Simulate a server error.
			http.Error(w, "go away", 404)
		} else {
			// Simulate a proper response.
			h := w.Header()
			h["Content-Type"] = []string{"text/x-vcard; charset=utf-8"}
			fmt.Fprintln(w, testCard)
		}
	}))

	local := onet.NewTCPTest(cothority.Suite)
	defer local.CloseAll()

	_, roster, _ := local.GenTree(3, true)

	c := evoting.NewClient()
	c.LookupURL = s.URL

	_, err := c.LookupSciper(roster, "")
	require.NotNil(t, err)
	_, err = c.LookupSciper(roster, "12345")
	require.NotNil(t, err)
	_, err = c.LookupSciper(roster, "1234567")
	require.NotNil(t, err)
	_, err = c.LookupSciper(roster, "000000")
	require.NotNil(t, err)
	vcard, err := c.LookupSciper(roster, "107537")
	require.Nil(t, err)
	require.Equal(t, "Martin Vetterli", vcard.FullName)
	require.Equal(t, "TYPE=INTERNET:martin.vetterli@epfl.ch", vcard.Email)
	s.Close()
}
