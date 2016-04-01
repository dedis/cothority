// Simple protocols to manage a simulation.
//
// Count calls on all children to return a count, waiting for 1 second of silence
// before giving up.
//
// Broadcast contacts all children to connect everybody else, thus setting up
// connections between everybody.
//
// CloseAll sends a message through the tree to close all hosts. This should
// only be used in simulation.
package manage
