/*
Package protocol contains an example demonstrating how to write a
protocol and a simulation.

The protocol has two messages:
	- Announce which is sent from the root down the tree
	- Reply which is sent back up to the root

If you want to add other messages, be sure to follow the way Announce and
StructAnnounce are set up.

A simple protocol uses four files:
- struct.go defines the messages sent around
- protocol.go defines the actions for each message
- protocol_test.go tests the protocol in a local test
- simulation.go tests the protocol on distant platforms like deterlab
*/
package protocol
