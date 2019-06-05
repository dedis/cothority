package lib

import (
	"testing"
)

func TestCombinationAnds(t *testing.T) {

	var tests = []struct {
		name     string
		m        int
		input    []string
		expected []string
	}{
		{"empty list m=-1", -1, []string{}, []string{}},
		{"empty list m=0", 0, []string{}, []string{}},
		{"empty list m=1", 1, []string{}, []string{}},
		{"empty list m=99", 99, []string{}, []string{}},
		{"standard m=-1", -1, []string{"A", "B", "C", "D"}, []string{}},
		{"standard m=0", 0, []string{"A", "B", "C", "D"}, []string{}},
		{"standard m=1", 1, []string{"A", "B", "C", "D"}, []string{
			"A", 
			"B", 
			"C", 
			"D",
		}},
		{"standard m=2", 2, []string{"A", "B", "C", "D"}, []string{
			"A & B", 
			"A & C", 
			"A & D", 
			"B & C", 
			"B & D", 
			"C & D",
		}},
		{"standard m=4", 4, []string{"A", "B", "C", "D"}, []string{
			"A & B & C & D",
		}},
		{"standard m=5", 5, []string{"A", "B", "C", "D"}, []string{}},
		{"standard m=99", 99, []string{"A", "B", "C", "D"}, []string{}},
		{"big m=4", 4, []string{"A", "B", "C", "D", "E", "F"}, []string{
			"A & B & C & D",
			"A & B & C & E",
			"A & B & C & F",
			"A & B & D & E",
			"A & B & D & F",
			"A & B & E & F",
			"A & C & D & E",
			"A & C & D & F",
			"A & C & E & F",
			"A & D & E & F",
			"B & C & D & E",
			"B & C & D & F",
			"B & C & E & F",
			"B & D & E & F",
			"C & D & E & F",
		}},
		{"with duplicate m=2", 2, []string{"A", "B", "A", "C", "D", "D"}, []string{
			"A & B", 
			"A & C", 
			"A & D", 
			"B & C", 
			"B & D", 
			"C & D",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CombinationAnds(tt.input, tt.m)
			for _, el := range tt.expected {
				if !contains(result, el) {
					t.Errorf("was expecting to find \"%s\"\nExpecting:\t%#v\nGot:\t%#v", el, tt.expected, result)
				}
			}
			if len(tt.expected) != len(result) {
				t.Errorf("sizes do not match (expected %d, found %d)\nExpecting:\t%#v\nGot:\t%#v", len(tt.expected), len(result), tt.expected, result)
			}
		})
	}
}

func TestPrependToEach(t *testing.T) {
	input := []string{"Hello", "world", "12"}
	expected := []string{"pre & Hello", "pre & world", "pre & 12"}
	result := prependAndToEach(input, "pre")
	for _, el := range expected {
		if !contains(result, el) {
			t.Errorf("was expecting to find \"%s\"\nExpecting:\t%#v\nGot:\t%#v", el, expected, result)
		}
	}
	if len(expected) != len(result) {
		t.Errorf("sizes do not match (expected %d, found %d)\nExpecting:\t%#v\nGot:\t%#v", len(expected), len(result), expected, result)
	}
}

// Helper method to check if an element is present in an array
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}
