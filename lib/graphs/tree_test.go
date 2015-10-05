package graphs

import (
	"fmt"
	"testing"
)

func TestLocalTree(test *testing.T) {
	nodes := []string{"1st", "2nd", "3rd", "4th", "5th", "6th", "7th", "8th"}
	fmt.Printf("Traversing tree for nodes %v \n", nodes)
	t := CreateLocalTree(nodes, 3)
	fmt.Printf(t.String(0))
	fmt.Printf("Depth = %d\n", Depth(t))
}
