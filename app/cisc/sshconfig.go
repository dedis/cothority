package main

import (
	"io/ioutil"
	"os"
	"strings"
)

/*
A very basic ssh-config file reader and writer. It can handle comments in
front of the 'Host' commands. When reading and writing, it is not guaranteed
that the file stays the same.
- sections always start with comment followed by "Host"
- the configuration-part of a host has to start with a tab
*/

// SSHConfig holds all host-definitions
type SSHConfig struct {
	Host []*SSHHost
}

// NewSSHConfig takes a string which represents the ssh-config files, parses
// it and returns an SSHConfig
func NewSSHConfig(str string) *SSHConfig {
	sc := &SSHConfig{}
	name := ""
	var cfgs, cmts []string
	// 0: comment; 1: config
	state := 0
	for _, s := range strings.Split(str, "\n") {
		read := false
		for !read {
			switch state {
			case 0:
				if has, str := prefStr(s, "Host"); has {
					name = str
					read = true
					state = 1
				} else {
					if has, str := prefStr(s, "#"); has {
						cmts = append(cmts, str)
					}
					read = true
				}
			case 1:
				if has, str := prefStr(s, "\t"); has {
					cfgs = append(cfgs, str)
					read = true
				} else {
					host := NewSSHHost(name, cfgs...)
					host.AddComments(cmts...)
					sc.Host = append(sc.Host, host)
					name = ""
					cmts = []string{}
					cfgs = []string{}
					state = 0
				}
			}
		}
	}
	if name != "" {
		host := NewSSHHost(name, cfgs...)
		host.AddComments(cmts...)
		sc.Host = append(sc.Host, host)
	}
	return sc
}

// NewSSHConfigFromFile opens the file and reads the config
func NewSSHConfigFromFile(name string) (*SSHConfig, error) {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		if os.IsNotExist(err) {
			return &SSHConfig{}, nil
		}
		return nil, err
	}
	return NewSSHConfig(string(b)), nil
}

// String can be used to return a valid ssh-config-file
func (s *SSHConfig) String() string {
	var str []string
	for _, h := range s.Host {
		str = append(str, h.String())
	}
	return strings.Join(str, "\n")
}

// NewHost adds a host
func (s *SSHConfig) AddHost(h *SSHHost) {
	s.Host = append(s.Host, h)
}

// DelHost searches for the host and removes it
func (s *SSHConfig) DelHost(name string) {
	var hosts []*SSHHost
	for _, h := range s.Host {
		if h.Name != name {
			hosts = append(hosts, h)
		}
	}
	s.Host = hosts
}

// SSSHost is one part of the config-file. It starts with an eventual comment
// followed by the name and the configuration.
type SSHHost struct {
	Comment []string
	Name    string
	Config  []string
}

// NewSSHHost returns an SSHHost from a name, comment and configuration.
func NewSSHHost(name string, conf ...string) *SSHHost {
	return &SSHHost{
		Name:   name,
		Config: conf,
	}
}

// AddComment adds a comment
func (s *SSHHost) AddComment(cmt string) {
	s.Comment = append(s.Comment, cmt)
}

// AddComments adds multiple lines of comments
func (s *SSHHost) AddComments(cmts ...string) {
	for _, cmt := range cmts {
		s.AddComment(cmt)
	}
}

// AddConfig adds a configuration-line
func (s *SSHHost) AddConfig(cfg string) {
	s.Config = append(s.Config, cfg)
}

// AddConfigs adds multiple configurations
func (s *SSHHost) AddConfigs(cfgs ...string) {
	for _, cfg := range cfgs {
		s.AddConfig(cfg)
	}
}

// String returns one part of an ssh-configuration.
func (s *SSHHost) String() string {
	var ret []string
	for _, c := range s.Comment {
		ret = append(ret, "# "+c)
	}
	ret = append(ret, "Host "+s.Name)
	for _, c := range s.Config {
		ret = append(ret, "\t"+c)
	}
	return strings.Join(ret, "\n") + "\n"
}

// prefStr helps when parsing the ssh-config-file by returning whether a string
// strats with a certain prefix and returns the string stripped by that
// prefix and eventual leading or following whitespaces.
func prefStr(str, prefix string) (bool, string) {
	hp := strings.HasPrefix(str, prefix)
	if hp {
		str = strings.TrimPrefix(str, prefix)
	}
	str = strings.TrimSpace(str)
	return hp, str
}
