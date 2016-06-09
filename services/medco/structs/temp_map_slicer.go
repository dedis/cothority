package medco_structs

import "github.com/dedis/crypto/abstract"

type KeyValCV struct {
	Key TempID
	Val CipherVector
}

func MapToSliceCV(m map[TempID]CipherVector) []KeyValCV {
	s := make([]KeyValCV, 0, len(m))
	for k, v := range m {
		s = append(s, KeyValCV{Key: k, Val: v})
	}
	return s
}

func SliceToMapCV(s []KeyValCV) map[TempID]CipherVector {
	m := make(map[TempID]CipherVector, len(s))
	for _,kv := range s {
		m[kv.Key] = kv.Val
	}
	return m
}

type KeyValGACV struct {
	Key GroupingAttributes
	Val CipherVector
}

func MapToSliceGACV(m map[GroupingAttributes]CipherVector) []KeyValGACV {
	s := make([]KeyValGACV, 0, len(m))
	for k, v := range m {
		s = append(s, KeyValGACV{Key: k, Val: v})
	}
	return s
}

func SliceToMapGACV(s []KeyValGACV) map[GroupingAttributes]CipherVector {
	m := make(map[GroupingAttributes]CipherVector, len(s))
	for _,kv := range s {
		m[kv.Key] = kv.Val
	}
	return m
}

type KeyValSPoint struct {
	Key TempID
	Val []abstract.Point
}

func MapToSliceSPoint(m map[TempID][]abstract.Point) []KeyValSPoint {
	s := make([]KeyValSPoint, 0, len(m))
	for k, v := range m {
		s = append(s, KeyValSPoint{Key: k, Val: v})
	}
	return s
}

func SliceToMapSPoint(s []KeyValSPoint) map[TempID][]abstract.Point {
	m := make(map[TempID][]abstract.Point, len(s))
	for _,kv := range s {
		m[kv.Key] = kv.Val
	}
	return m
}