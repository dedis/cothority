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

type KeyValGKCV struct {
	Key GroupingKey
	Val CipherVector
}

func MapToSliceGKCV(m map[GroupingKey]CipherVector) []KeyValGKCV {
	s := make([]KeyValGKCV, 0, len(m))
	for k, v := range m {
		s = append(s, KeyValGKCV{Key: k, Val: v})
	}
	return s
}

func SliceToMapGKCV(s []KeyValGKCV) map[GroupingKey]CipherVector {
	m := make(map[GroupingKey]CipherVector, len(s))
	for _,kv := range s {
		m[kv.Key] = kv.Val
	}
	return m
}

type KeyValGKGA struct {
	Key GroupingKey
	Val GroupingAttributes
}

func MapToSliceGKGA(m map[GroupingKey]GroupingAttributes) []KeyValGKGA {
	s := make([]KeyValGKGA, 0, len(m))
	for k, v := range m {
		s = append(s, KeyValGKGA{Key: k, Val: v})
	}
	return s
}

func SliceToMapGKGA(s []KeyValGKGA) map[GroupingKey]GroupingAttributes {
	m := make(map[GroupingKey]GroupingAttributes, len(s))
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