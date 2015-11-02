package cliutils

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// This file handles manipulations of IP address with ports
// Like checking if an address contains a port, adding one etc

var addressRegexp *regexp.Regexp

func init() {
	addressRegexp = regexp.MustCompile(`^[^:]*(:)(\d{1,5})?$`)
}

// Checks if an address contains a port. If it does not, it add the
// given port to that and returns the new address. If it does, it returns
// directly. Both operation checks if the port is correct.
func VerifyPort(address string, port int) (string, error) {
	p := strconv.Itoa(port)
	subs := addressRegexp.FindStringSubmatch(address)
	switch{
	case len(subs) == 0:
		// address does not contain a port
		return address + ":" + p, checkPort(port)
	case len(subs) == 3 && subs[2] == "":
		// we got a addres: style ..??
		return address + p, checkPort(port)
	case len(subs) == 3:
		// we got full address already address:port
		sp, err := strconv.Atoi(subs[2])
		if err != nil {
			return address, errors.New("Not a valid port-number given")
		}
		return address, checkPort(sp)
	}
	return address, errors.New("Could not anaylze address")
}

// Returns the global-binding address
func GlobalBind(address string) (string, error) {
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		return "", errors.New("Not a host:port address")
	}
	return "0.0.0.0:" + addr[1], nil
}

// Simply returns an error if the port is invalid
func checkPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("Port number invalid %d !", port)
	}
	return nil
}
