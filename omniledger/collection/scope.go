package collection

import "crypto/sha256"

// mask

type mask struct {
	value []byte
	bits  int
}

// Private methods

func (m *mask) match(path [sha256.Size]byte, bits int) bool {
	if bits < m.bits {
		return match(path[:], m.value, bits)
	}
	return match(path[:], m.value, m.bits)
}

// scope

type scope struct {
	masks []mask
	all   bool
}

// Methods

func (s *scope) All() {
	s.all = true
	s.masks = []mask{}
}

func (s *scope) None() {
	s.all = false
	s.masks = []mask{}
}

func (s *scope) Add(value []byte, bits int) {
	s.masks = append(s.masks, mask{value, bits})
}

// Private methods

func (s *scope) match(path [sha256.Size]byte, bits int) bool {
	if len(s.masks) == 0 {
		return s.all
	}

	for index := 0; index < len(s.masks); index++ {
		if s.masks[index].match(path, bits) {
			return true
		}
	}

	return false
}

func (s *scope) clone() (scope scope) {
	scope.masks = make([]mask, len(s.masks))
	copy(scope.masks, s.masks)
	scope.all = s.all

	return
}
