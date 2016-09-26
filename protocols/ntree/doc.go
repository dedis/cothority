// Package ntree implements a simple protocol where each node signs a message
// and the parent node verifies it.
// The leader (the root node) collects N standard individual signatures
// from the N witnesses using a tree.
// It is very similar to the "naive" scheme where the leader directly sends the
// message to be signed directly to its children. Th naive scheme is the the
// same protocol but with a 1-level tree.
// It can be used to compare both naive and ntree with CoSi.
package ntree
