package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/stretchr/testify/require"
)

var testSuite = pairing.NewSuiteBn256()

// TestMain_Check checks if the CLI command check works correctly
func TestMain_Check(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	os.Chdir(tmp)

	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	_, roster, _ := local.GenTree(10, true)

	publicToml := path.Join(tmp, "public.toml")

	cliApp := createApp()
	require.NotNil(t, cliApp)

	group := &app.Group{Roster: roster}
	err := group.Save(testSuite, publicToml)
	require.NoError(t, err)
	err = cliApp.Run([]string{"", "check", "-g", publicToml, "--detail"})
	require.NoError(t, err)
}

// TestMain_Sign checks if the CLI commands sign and verify work correctly
func TestMain_Sign(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	os.Chdir(tmp)

	publicToml := path.Join(tmp, "public.toml")
	signatureFile := path.Join(tmp, "sig.json")

	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()
	_, roster, _ := local.GenTree(5, true)
	group := &app.Group{Roster: roster}
	err := group.Save(testSuite, publicToml)
	require.NoError(t, err)

	cliApp := createApp()
	require.NotNil(t, cliApp)

	// missing file to sign
	err = cliApp.Run([]string{"", "sign"})
	require.Error(t, err)

	// incorrect file to sign
	err = cliApp.Run([]string{"", "sign", "abc"})
	require.Error(t, err)

	// stdout output
	err = cliApp.Run([]string{"", "sign", "-g", publicToml, publicToml})
	require.NoError(t, err)

	// file output
	err = cliApp.Run([]string{"", "sign", "-g", publicToml, "-o", signatureFile, publicToml})
	require.NoError(t, err)

	// verify the output
	err = cliApp.Run([]string{"", "verify", "-g", publicToml, "-s", signatureFile, publicToml})
	require.NoError(t, err)

	// missing file to verify
	err = cliApp.Run([]string{"", "verify"})
	require.Error(t, err)

	// wrong file to verify
	err = cliApp.Run([]string{"", "verify", "-g", publicToml, "-s", signatureFile, signatureFile})
	require.Error(t, err)
}
