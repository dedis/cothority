package edwards25519

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPoint_Marshal(t *testing.T) {
	p := Point{}
	require.Equal(t, "ed.Point", fmt.Sprintf("%s", p.MarshalID()))
}
