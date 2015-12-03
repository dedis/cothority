package graphs

import (
	"testing"
)

func TestLocalTree(test *testing.T) {
	nodes := []string{"1st", "2nd", "3rd", "4th", "5th", "6th", "7th", "8th"}
	test.Logf("Traversing tree for nodes %v \n", nodes)
	t := CreateLocalTree(nodes, 3)
	test.Logf(t.String(0))
	test.Logf("Depth = %d\n", Depth(t))
}
