// Package blkparser basically is an adaptation from a file at https://github.com/tsileo/blkparser
package blkparser

import (
	"crypto/sha256"
	"fmt"

	"github.com/dedis/cothority/lib/dbg"
)

// Get the Tx count, decode the variable length integer
// https://en.bitcoin.it/wiki/Protocol_specification#Variable_length_integer
func DecodeVariableLengthInteger(raw []byte) (cnt int, cnt_size int) {
	if raw[0] < 0xfd {
		return int(raw[0]), 1
	}
	cnt_size = 1 + (2 << (2 - (0xff - raw[0])))
	if len(raw) < 1+cnt_size {
		return
	}

	res := uint64(0)
	for i := 1; i < cnt_size; i++ {
		res |= (uint64(raw[i]) << uint64(8*(i-1)))
	}

	cnt = int(res)
	return
}

func GetShaString(data []byte) (res string) {
	sha := sha256.New()
	if _, err := sha.Write(data[:]); err != nil {
		dbg.Error("Failed to hash data", err)
	}
	tmp := sha.Sum(nil)
	sha.Reset()
	if _, err := sha.Write(tmp); err != nil {
		dbg.Error("Failed to hash data", err)
	}
	hash := sha.Sum(nil)
	res = HashString(hash)
	return
}

func HashString(data []byte) (res string) {
	for i := 0; i < 32; i++ {
		res += fmt.Sprintf("%02x", data[31-i])
	}
	return
}
