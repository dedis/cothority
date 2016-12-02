package sda

import "testing"

func TestNewWebService(t *testing.T) {
	c := NewLocalConode(0)
	defer c.Close()
	c.serviceManager.services
}
