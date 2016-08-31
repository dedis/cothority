package timestamp

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func TestRequestBuffer(t *testing.T) {
	rb := &requestPool{}
	for i := 0; i < 10; i++ {
		rb.Add([]byte("data_" + strconv.Itoa(i)))
	}
	//fmt.Println(string(rb.requestData[0]))
	assert.Equal(t, len(rb.requestData), 10)
	rb.reset()
	assert.Equal(t, len(rb.requestData), 0)
}
