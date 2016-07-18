/*
This is a test-function for the external-methods. Every call in here
should go through the interface created in `external.go`.
*/
package identity

import (
	"testing"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestExternal_CreateIdentity(t *testing.T) {
	//t.Skip()
	l := sda.NewLocalTest()
	_, el, _ := l.GenTree(3, true, true, true)
	defer l.CloseAll()

	// This has to be done in Java, too.
	c := NewIdentity(el, 50, "one")

	// Now instead of calling the `CreateIdentity` of
	// `Service`, you will have to create a request with
	// whatever method you chose in `external.go`
	log.ErrFatal(c.CreateIdentity())

	// Check we're in the configuration
	assert.NotNil(t, c.Config)
}

// Here you can start implementing the other methods from
// `api_test.go`.
