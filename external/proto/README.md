Some of these are manually managed. Search for "// MANUAL" to see which ones.
Currently the list of manual ones are:
* network.proto
* onet.proto
* skipchain.proto

The rest of these are derived from the matching $dir/proto.go file. (i.e. trie.proto ->
../../byzcoin/trie/proto.go).

The "make proto" target in the root Makefile creates these.
