package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var ssh_config string = `#Comment 1
Host alias1
	Port 521
	HostName host1
	#Comment 2
Host alias2
	Port 32

# Comment 3
# Comment 3.1
Host a3
	Port 223`

var ssh_config_out string = `# Comment 1
Host alias1
	Port 521
	HostName host1
	#Comment 2

Host alias2
	Port 32

# Comment 3
# Comment 3.1
Host a3
	Port 223
`

var ssh_host string = `# Aliased host 4
Host alias4
	Port 21
	HostName host4
`

var ssh_host2 string = `# Aliased host 5
# More comments
Host alias5
	Port 21
	HostName host5
`

func TestNewSSHConfig(t *testing.T) {
	sc := NewSSHConfig(ssh_config)
	assert.Equal(t, ssh_config_out, sc.String())
}

func TestSSHHost_AddComment(t *testing.T) {

	host := NewSSHHost("alias4", "Port 21")
	host.AddComment("Aliased host 4")
	host.AddConfig("HostName host4")
	assert.Equal(t, ssh_host, host.String())

	host = NewSSHHost("alias5")
	host.AddComments("Aliased host 5", "More comments")
	host.AddConfigs("Port 21", "HostName host5")
	assert.Equal(t, ssh_host2, host.String())
}

func TestSSHConfig_DelHost(t *testing.T) {
	sc := NewSSHConfig(ssh_config)

	assert.Equal(t, 3, len(sc.Host))
	sc.DelHost("one")
	assert.Equal(t, 3, len(sc.Host))
	sc.DelHost("a3")
	assert.Equal(t, 2, len(sc.Host))
}

func TestSSHConfig_SearchHost(t *testing.T) {
	sc := NewSSHConfig(ssh_config)
	host := sc.SearchHost("alias1")
	assert.NotNil(t, host)
	if host.GetConfig("HostName") != "host1" {
		t.Fatal("Didn't receive correct host")
	}
	host = sc.SearchHost("alien1")
	assert.Nil(t, host, "Shouldn't find alien1")
}

func TestSSHHost_GetConfig(t *testing.T) {
	sc := NewSSHConfig(ssh_config)
	host := sc.SearchHost("alias1")
	cfg := host.GetConfig("HostName")
	assert.Equal(t, "host1", cfg)
	cfg = host.GetConfig("AlienName")
	assert.Equal(t, "", cfg)
}

func TestSSHConfig_ConvertAliasToHostname(t *testing.T) {
	sc := NewSSHConfig(ssh_config)
	assert.Equal(t, "host1", sc.ConvertAliasToHostname("alias1"))
	assert.Equal(t, "alien1", sc.ConvertAliasToHostname("alien1"))
}
