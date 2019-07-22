package lib

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/v3/log"

	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
	"gopkg.in/urfave/cli.v1"
)

// StringToDarcID converts a string representation of a DARC to a byte array
func StringToDarcID(id string) ([]byte, error) {
	if id == "" {
		return nil, errors.New("no string given")
	}
	if strings.HasPrefix(id, "darc:") {
		id = id[5:]
	}
	return hex.DecodeString(id)
}

// StringToEd25519Buf converts the string representation of an ed25519 key to a
// byte array
func StringToEd25519Buf(pub string) ([]byte, error) {
	if pub == "" {
		return nil, errors.New("no string given")
	}
	if strings.HasPrefix(pub, "ed25519:") {
		pub = pub[8:]
	}
	return hex.DecodeString(pub)
}

// GetDarcByString returns a DARC given its ID as a string
func GetDarcByString(cl *byzcoin.Client, id string) (*darc.Darc, error) {
	xrep, err := StringToDarcID(id)
	if err != nil {
		return nil, err
	}
	return GetDarcByID(cl, xrep)
}

// GetDarcByID returns a DARC given its ID as a byte array
func GetDarcByID(cl *byzcoin.Client, id []byte) (*darc.Darc, error) {
	pr, err := cl.GetProofFromLatest(id)
	if err != nil {
		return nil, err
	}

	p := &pr.Proof

	vs, cid, _, err := p.Get(id)
	if err != nil {
		return nil, fmt.Errorf("could not find darc for %x", id)
	}
	if cid != byzcoin.ContractDarcID {
		return nil, fmt.Errorf("unexpected contract %v, expected a darc", cid)
	}

	d, err := darc.NewFromProtobuf(vs)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// ExportTransaction will redirect the transaction to stdout. It must be made
// sure that no other print is done, else the stdout is not usable.
func ExportTransaction(tx byzcoin.ClientTransaction) error {
	// When exporting, we must not pass SignerCounter, SignerIdentities and
	// Signatures. Hence, we build a new list of instructions by ommiting
	// those parameters. We can't edit current ones because those are not
	// pointers.
	instrs := make([]byzcoin.Instruction, len(tx.Instructions))
	for i, instr := range tx.Instructions {
		instrs[i] = byzcoin.Instruction{
			InstanceID: instr.InstanceID,
			Spawn:      instr.Spawn,
			Invoke:     instr.Invoke,
			Delete:     instr.Delete,
		}
	}
	tx.Instructions = instrs
	buf, err := protobuf.Encode(&tx)
	if err != nil {
		return errors.New("failed to encode tx: " + err.Error())
	}
	reader := bytes.NewReader(buf)
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.New("failed to copy to stdout: " + err.Error())
	}
	return nil
}

// FindRecursivefBool recursively check the context to find argname
func FindRecursivefBool(argname string, c *cli.Context) bool {
	for c != nil {
		if c.IsSet(argname) {
			return c.Bool(argname)
		}
		c = c.Parent()
	}
	return false
}

// CombinationAnds returns a list that contains AND groups of M elements. It
// allows to compute rule of kind "M out of N". Each single element and each
// group is surrounded by parenthesis.
//
// If the input list is ["A", "B", "C", "D | E"], and M = 2, it will return the
// following list:
//
// [ "((A) & (B))",
//   "((A) & (C))",
//   "((A) & (D | E))",
//   "((B) & (C))",
//   "((B) & (D | E))",
//   "((C) & (D | E))" ]
//
// Duplicates in the input list are removed.
//
func CombinationAnds(list []string, m int) []string {
	if m <= 0 {
		return []string{}
	}
	list = unique(list)
	list = upperSiblings(m, 0, list)
	for i := range list {
		list[i] = "(" + list[i] + ")"
	}
	return list
}

// WaitPropagation contacts all nodes in the cl.Roster until they all
// have the same latest block. If there is an error when calling
// `GetProof`, the error will be ignored. This helps when waiting
// for the propagation, but only a subset of the nodes are actually
// participating in the consensus.
func WaitPropagation(c *cli.Context, cl *byzcoin.Client) error {
	if !c.GlobalBool("wait") {
		return nil
	}

	var sb skipchain.SkipBlock
	sb.SkipBlockFix = &skipchain.SkipBlockFix{}
searchLatest:
	for i := 0; i < 100; i++ {
		log.Lvl2("Starting search")
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		for node := range cl.Roster.List {
			log.Lvl2("Searching node", node)
			if err := cl.UseNode(node); err != nil {
				return err
			}
			_, err := cl.GetProof(make([]byte, 32))
			if err != nil {
				log.Warn("error while searching for node - ignoring")
				//continue searchLatest
			}
			if cl.Latest.Index > sb.Index {
				sb = *cl.Latest
				log.Lvl2("Found new block:", sb.Index)
				cc, err := cl.GetChainConfig()
				if err != nil {
					return err
				}
				cl.Roster = cc.Roster
				if node > 0 {
					continue searchLatest
				}
			} else if cl.Latest.Index < sb.Index {
				log.Lvlf2("Node %d returned earlier block: %d", node, cl.Latest.Index)
				continue searchLatest
			}
			log.Lvl2("Node", node, "returned same block as other nodes")
		}
		return nil
	}
	return errors.New("didn't get the same blocks from everybody within 10 seconds")
}

// We are recursively building the leaves of a tree that contains every
// M combination at each level.
// The following illustrate such tree for 4 elements and up to M = 3.
//
//  M = 1           A         B     C     D
//                  |         |     |
//                / | \      / \     \
//               /  |  \    /   \     \
//  M = 2       AB  AC AD   BC  BD    CD
//              --    \      |
//             /  \    \     |
//  M = 3    ABC  ABD  ACD  BCD
//
// From M = 3, building the leaves that start with "A" is done by prepending
// "A" to all the sublings element of the previous level (M = 2).
// In that case, "A" must be prepended to its upper siblings elements, which
// are "BC", "BD", and "CD".
// This process is recursively done for every element until the trivial case,
// where M =1. Making a combination of 1 with one element is the element itself.
// Level indicates the depth (M) and index the element (0 = "A", 1 = "B", ..)
func upperSiblings(level int, index int, elements []string) []string {
	res := make([]string, 0)
	if level == 1 {
		for i := index; i <= len(elements)-level; i++ {
			res = append(res, "("+elements[i]+")")
		}
		return res
	}
	for i := index; i <= len(elements)-level; i++ {
		// Get upper level (level-1) sibling (i+1) elements
		subRes := upperSiblings(level-1, i+1, elements)
		subRes = prependAndToEach(subRes, "("+elements[i]+")")
		res = append(res, subRes...)
	}
	return res
}

// This utility method prepends a given string to every element of the given
// list with an " & " and returns it
func prependAndToEach(list []string, prefix string) []string {
	res := make([]string, len(list))
	for i := range list {
		res[i] = prefix + " & " + list[i]
	}
	return res
}

// removes duplicate from a slice and return a new list
func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, ok := keys[entry]; !ok {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
