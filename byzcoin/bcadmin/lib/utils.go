package lib

import (
	"bytes"
	"encoding/hex"
	"io"
	"math/big"
	"os"
	"strings"

	"golang.org/x/xerrors"

	"go.dedis.ch/kyber/v3/util/random"

	"github.com/urfave/cli"
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
	"go.dedis.ch/protobuf"
)

// StringToDarcID converts a string representation of a DARC to a byte array
func StringToDarcID(id string) ([]byte, error) {
	if id == "" {
		return nil, xerrors.New("no string given")
	}
	return hex.DecodeString(strings.TrimPrefix(id, "darc:"))
}

// StringToEd25519Buf converts the string representation of an ed25519 key to a
// byte array
func StringToEd25519Buf(pub string) ([]byte, error) {
	if pub == "" {
		return nil, xerrors.New("no string given")
	}
	return hex.DecodeString(strings.TrimPrefix(pub, "ed25519:"))
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
		return nil, xerrors.Errorf("could not find darc for %x", id)
	}
	if cid != byzcoin.ContractDarcID {
		return nil, xerrors.Errorf("unexpected contract %v, expected a darc", cid)
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
		return xerrors.Errorf("failed to encode tx: %v", err)
	}
	reader := bytes.NewReader(buf)
	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return xerrors.Errorf("failed to copy to stdout: %v", err)
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

// WaitPropagation checks if the "--wait" argument is given, and only if this
// is true, it will make sure that all nodes have a coherent view of the chain.
func WaitPropagation(c *cli.Context, cl *byzcoin.Client) error {
	if !c.GlobalBool("wait") {
		return nil
	}

	return cl.WaitPropagation(-1)
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

// RandString return a random string of length n
func RandString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bigN := big.NewInt(int64(len(letters)))
	b := make([]byte, n)
	r := random.New()
	for i := range b {
		x := int(random.Int(bigN, r).Int64())
		b[i] = letters[x]
	}
	return string(b)
}
