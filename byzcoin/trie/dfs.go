package trie

import "errors"

type nodeProcessor interface {
	OnEmpty(n emptyNode, k, v []byte) error
	OnLeaf(n leafNode, k, v []byte) error
	OnInterior(n interiorNode, k, v []byte) error
}

// dfs is a depth first traversal. On every node, the corresponding function in
// nodeProcessor is called. If an error is returned, then the traversal stops.
func (t *Trie) dfs(p nodeProcessor, nodeKey []byte, b bucket) error {
	nodeVal := b.Get(nodeKey)
	if len(nodeVal) == 0 {
		return errors.New("node key does not exist in copyTo")
	}
	switch nodeType(nodeVal[0]) {
	case typeEmpty:
		node, err := decodeEmptyNode(nodeVal)
		if err != nil {
			return err
		}
		return p.OnEmpty(node, node.hash(t.nonce), nodeVal)
	case typeLeaf:
		node, err := decodeLeafNode(nodeVal)
		if err != nil {
			return err
		}
		return p.OnLeaf(node, node.hash(t.nonce), nodeVal)
	case typeInterior:
		node, err := decodeInteriorNode(nodeVal)
		if err != nil {
			return err
		}
		if err := p.OnInterior(node, node.hash(), nodeVal); err != nil {
			return err
		}
		if err := t.dfs(p, node.Left, b); err != nil {
			return err
		}
		if err := t.dfs(p, node.Right, b); err != nil {
			return err
		}
		return nil
	}
	return errors.New("invalid node type")
}

type copyNodeProcessor struct {
	target bucket
}

func (p *copyNodeProcessor) OnEmpty(n emptyNode, k, v []byte) error {
	return p.target.Put(k, append([]byte{}, v...))
}

func (p *copyNodeProcessor) OnLeaf(n leafNode, k, v []byte) error {
	return p.target.Put(k, append([]byte{}, v...))
}

func (p *copyNodeProcessor) OnInterior(n interiorNode, k, v []byte) error {
	return p.target.Put(k, append([]byte{}, v...))
}

type countNodeProcessor struct {
	total  int
	leaves []leafNode
}

func (p *countNodeProcessor) OnEmpty(n emptyNode, k, v []byte) error {
	p.total++
	return nil
}

func (p *countNodeProcessor) OnLeaf(n leafNode, k, v []byte) error {
	p.total++
	p.leaves = append(p.leaves, n)
	return nil
}

func (p *countNodeProcessor) OnInterior(n interiorNode, k, v []byte) error {
	p.total++
	return nil
}
