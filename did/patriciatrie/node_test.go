package patriciatrie

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func Test_LeafNodeHashEvenLength(t *testing.T) {
	valueEncoding, _ := rlp.EncodeToBytes([][]byte{[]byte("bar")})
	snode := shortNode{
		remainingPath: hexToNibbles([]byte("foo")),
		value:         valuenode(valueEncoding),
		flag:          EVEN_LEAF,
	}
	hash := snode.hash()
	require.Equal(t, hash, snode)

	expectedEncoding, err := hex.DecodeString("cb8420666f6f85c483626172")
	require.NoError(t, err)
	rlpEncoding, err := rlp.EncodeToBytes(snode)
	require.NoError(t, err)
	require.Equal(t, expectedEncoding, rlpEncoding)
}

func Test_LeafNodeHashOddLength(t *testing.T) {
	valueEncoding, _ := rlp.EncodeToBytes([][]byte{[]byte("bar")})
	path := hexToNibbles([]byte("bar"))
	snode := shortNode{
		remainingPath: path[1:],
		value:         valuenode(valueEncoding),
		flag:          ODD_LEAF,
	}
	hash := snode.hash()
	require.Equal(t, hash, snode)

	expectedEncoding, err := hex.DecodeString("ca8332617285c483626172")
	require.NoError(t, err)
	rlpEncoding, err := rlp.EncodeToBytes(snode)
	require.NoError(t, err)
	require.Equal(t, expectedEncoding, rlpEncoding)
}

func Test_BranchNodeHash(t *testing.T) {
	// foo: bar
	// foobar: bar

	path := hexToNibbles([]byte("bar"))
	valueEncoding, _ := rlp.EncodeToBytes([][]byte{[]byte("bar")})

	snode := &shortNode{
		remainingPath: path[1:],
		value:         valuenode(valueEncoding),
		flag:          ODD_LEAF,
	}
	bnode := branchNode{}
	bnode[path[0]] = snode
	bnode[16] = valuenode(valueEncoding)

	hash := bnode.hash()
	require.IsType(t, hashnode{}, hash)

	expectedHash, err := hex.DecodeString("995a6cb22ca62917fceb6f4092f2f13f044b8042f4eba9716f6e6438ba4adc91")
	require.NoError(t, err)
	require.Equal(t, hashnode(expectedHash), hash)
}

func Test_BranchNodeHashEmbedded(t *testing.T) {
	// foo: bar
	// foobar: bar

	path := hexToNibbles([]byte("bar"))
	valueEncoding, _ := rlp.EncodeToBytes([][]byte{[]byte("b")})

	snode := &shortNode{
		remainingPath: path[1:],
		value:         valuenode(valueEncoding),
		flag:          ODD_LEAF,
	}
	bnode := branchNode{}
	bnode[path[0]] = snode
	bnode[16] = valuenode(valueEncoding)

	hash := bnode.hash()
	require.IsType(t, branchNode{}, hash)

	expectedEncoding, err := hex.DecodeString("da808080808080c78332617282c16280808080808080808082c162")
	require.NoError(t, err)

	rlpEncoding, err := rlp.EncodeToBytes(bnode)
	require.Equal(t, expectedEncoding, rlpEncoding)
}

func Test_ExtendedNodeHashEven(t *testing.T) {
	// foo: bar
	// foobar: bar

	path := hexToNibbles([]byte("bar"))
	path2 := hexToNibbles([]byte("foo"))
	valueEncoding, _ := rlp.EncodeToBytes([][]byte{[]byte("bar")})

	snode := &shortNode{
		remainingPath: path[1:],
		value:         valuenode(valueEncoding),
		flag:          ODD_LEAF,
	}
	bnode := branchNode{}
	bnode[path[0]] = snode
	bnode[16] = valuenode(valueEncoding)

	hash := bnode.hash()

	enode := shortNode{
		remainingPath: path2,
		value:         hash,
		flag:          EVEN_EXT,
	}

	ehash := enode.hash()
	require.IsType(t, hashnode{}, ehash)

	expectedHash, err := hex.DecodeString("df1ea1f9390dacc30a1bd1affa848f9cf39a0ff49e0b7dd2280f31116f70dd27")
	require.NoError(t, err)
	require.Equal(t, hashnode(expectedHash), ehash)
}
