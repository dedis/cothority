package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/dedis/kyber/pairing"
	"github.com/dedis/onet"
	"github.com/dedis/onet/app"
	"github.com/dedis/onet/network"
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
	hosts, roster, _ := local.GenTree(10, true)

	publicToml := tmp + "/public.toml"

	cliApp := createApp()
	require.NotNil(t, cliApp)

	err := cliApp.Run([]string{"", "check", "-g", ""})
	require.Error(t, err)

	err = ioutil.WriteFile(tmp+"/error.toml", []byte("abc"), 0644)
	require.NoError(t, err)
	err = cliApp.Run([]string{"", "check", "-g", tmp + "/error.toml"})
	require.Error(t, err)

	group := &app.Group{Roster: &onet.Roster{List: []*network.ServerIdentity{}}}
	err = group.Save(testSuite, publicToml)
	require.NoError(t, err)
	err = cliApp.Run([]string{"", "check", "-g", publicToml})
	require.Error(t, err)

	group = &app.Group{Roster: roster}
	err = group.Save(testSuite, publicToml)
	require.NoError(t, err)
	err = cliApp.Run([]string{"", "check", "-g", publicToml, "--detail"})
	require.NoError(t, err)

	hosts[0].Close()
	err = cliApp.Run([]string{"", "check", "-g", publicToml})
	require.Error(t, err)
}

// TestMain_Sign checks if the CLI commands sign and verify work correctly
func TestMain_Sign(t *testing.T) {
	tmp, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmp)

	os.Chdir(tmp)

	publicToml := tmp + "/public.toml"
	signatureFile := tmp + "/sig.json"

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
