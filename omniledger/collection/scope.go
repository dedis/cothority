package collection

import csha256 "crypto/sha256"

// mask

type mask struct {
	value []byte
	bits  int
}

// Private methods

func (this *mask) match(path [csha256.Size]byte, bits int) bool {
	if bits < this.bits {
		return match(path[:], this.value, bits)
	} else {
		return match(path[:], this.value, this.bits)
	}
}

// scope

type scope struct {
	masks []mask
	all   bool
}

// Methods

func (this *scope) All() {
	this.all = true
	this.masks = []mask{}
}

func (this *scope) None() {
	this.all = false
	this.masks = []mask{}
}

func (this *scope) Add(value []byte, bits int) {
	this.masks = append(this.masks, mask{value, bits})
}

// Private methods

func (this *scope) match(path [csha256.Size]byte, bits int) bool {
	if len(this.masks) == 0 {
		return this.all
	}

	for index := 0; index < len(this.masks); index++ {
		if this.masks[index].match(path, bits) {
			return true
		}
	}

	return false
}

func (this *scope) clone() (scope scope) {
	scope.masks = make([]mask, len(this.masks))
	copy(scope.masks, this.masks)
	scope.all = this.all

	return
}
