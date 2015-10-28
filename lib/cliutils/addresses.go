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
func VerifyPort(address, port string) (string, error) {
	// address does not contains a port
	if subs := addressRegexp.FindStringSubmatch(address); len(subs) == 0 {
		return address + ":" + port, checkPort(port)
	} else if len(subs) == 3 && subs[2] == "" { // we got a addres: style ..??
		return address + port, checkPort(port)
	} else if len(subs) == 3 { // we got full address already address:port
		return address, checkPort(subs[2])
	}
	return address, errors.New("Could not anaylze address ><")
}

// Returns the global-binding address
func GlobalBind(address string)(string, error){
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		return "", errors.New("Not a host:port address")
	}
	return "0.0.0.0:" + addr[1], nil
}

// Simply returns an error if the port is invalid
func checkPort(port string) error {
	porti, err := strconv.Atoi(port)
	if err != nil {
		return err
	} else if porti < 1 || porti >= 65536 {
		return fmt.Errorf("Port number invalid %d !", porti)
	}
	return nil
}
