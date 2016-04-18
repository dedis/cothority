package sda

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_collectErrors(t *testing.T) {
	testMap := map[string]error{
		"127.0.0.1": errors.New("We are offline, sorry."),
		"127.0.0.2": errors.New("Timeout..."),
	}
	err := collectErrors("Error while contacting %s: %s\n", testMap)
	assert.NotNil(t, err, "Should create a valid error.")

	got := string(err.Error())
	want := "Error while contacting 127.0.0.1: We are offline, sorry.\n" +
		"Error while contacting 127.0.0.2: Timeout...\n"

	assert.Equal(t, got, want)
}
