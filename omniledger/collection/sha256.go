package collection

import "reflect"
import csha256 "crypto/sha256"
import "encoding/binary"

func sha256(item interface{}, items ...interface{}) [csha256.Size]byte {
	const (
		boolid = iota
		int8id
		int16id
		int32id
		int64id
		uint8id
		uint16id
		uint32id
		uint64id

		boolsliceid
		int8sliceid
		int16sliceid
		int32sliceid
		int64sliceid
		uint8sliceid
		uint16sliceid
		uint32sliceid
		uint64sliceid

		stringid
		arrayid
	)

	var size func(interface{}) int
	size = func(item interface{}) int {
		switch value := item.(type) {
		case bool:
			return 2
		case int8:
			return 2
		case int16:
			return 3
		case int32:
			return 5
		case int64:
			return 9
		case uint8:
			return 2
		case uint16:
			return 3
		case uint32:
			return 5
		case uint64:
			return 9
		case []bool:
			return 9 + len(value)
		case []int8:
			return 9 + len(value)
		case []int16:
			return 9 + 2*len(value)
		case []int32:
			return 9 + 4*len(value)
		case []int64:
			return 9 + 8*len(value)
		case []uint8:
			return 9 + len(value)
		case []uint16:
			return 9 + 2*len(value)
		case []uint32:
			return 9 + 4*len(value)
		case []uint64:
			return 9 + 8*len(value)
		case string:
			return 9 + len(value)
		}

		reflection := reflect.ValueOf(item)

		if reflection.Kind() == reflect.Slice {
			total := 9

			for index := 0; index < reflection.Len(); index++ {
				total += size(reflection.Index(index).Interface())
			}

			return total
		}

		panic("sha256() only accepts: bool, int8, int16, int32, int64, uint8, uint16, uint32, uint64 and string (or N-dimensional slices of those types, with N >= 1).")
	}

	alloc := size(item)
	for _, oitem := range items {
		alloc += size(oitem)
	}

	buffer := make([]byte, alloc)

	var write func([]byte, interface{}) []byte
	write = func(buffer []byte, item interface{}) []byte {
		switch value := item.(type) {
		case bool:
			buffer[0] = boolid
			if value {
				buffer[1] = 1
			} else {
				buffer[1] = 0
			}

			return buffer[2:]

		case int8:
			buffer[0] = int8id
			buffer[1] = byte(value)
			return buffer[2:]

		case int16:
			buffer[0] = int16id
			binary.BigEndian.PutUint16(buffer[1:], uint16(value))
			return buffer[3:]

		case int32:
			buffer[0] = int32id
			binary.BigEndian.PutUint32(buffer[1:], uint32(value))
			return buffer[5:]

		case int64:
			buffer[0] = int64id
			binary.BigEndian.PutUint64(buffer[1:], uint64(value))
			return buffer[9:]

		case uint8:
			buffer[0] = uint8id
			buffer[1] = byte(value)
			return buffer[2:]

		case uint16:
			buffer[0] = uint16id
			binary.BigEndian.PutUint16(buffer[1:], value)
			return buffer[3:]

		case uint32:
			buffer[0] = uint32id
			binary.BigEndian.PutUint32(buffer[1:], value)
			return buffer[5:]

		case uint64:
			buffer[0] = uint64id
			binary.BigEndian.PutUint64(buffer[1:], value)
			return buffer[9:]

		case []bool:
			buffer[0] = boolsliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				if value[index] {
					buffer[9+index] = 1
				} else {
					buffer[9+index] = 0
				}
			}

			return buffer[9+len(value):]

		case []int8:
			buffer[0] = int8sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				buffer[9+index] = byte(value[index])
			}

			return buffer[9+len(value):]

		case []int16:
			buffer[0] = int16sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint16(buffer[9+2*index:], uint16(value[index]))
			}

			return buffer[9+2*len(value):]

		case []int32:
			buffer[0] = int32sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint32(buffer[9+4*index:], uint32(value[index]))
			}

			return buffer[9+4*len(value):]

		case []int64:
			buffer[0] = int64sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint64(buffer[9+8*index:], uint64(value[index]))
			}

			return buffer[9+8*len(value):]

		case []uint8:
			buffer[0] = uint8sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))
			copy(buffer[9:], value)
			return buffer[9+len(value):]

		case []uint16:
			buffer[0] = uint16sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint16(buffer[9+2*index:], value[index])
			}

			return buffer[9+2*len(value):]

		case []uint32:
			buffer[0] = uint32sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint32(buffer[9+4*index:], value[index])
			}

			return buffer[9+4*len(value):]

		case []uint64:
			buffer[0] = uint64sliceid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))

			for index := 0; index < len(value); index++ {
				binary.BigEndian.PutUint64(buffer[9+8*index:], uint64(value[index]))
			}

			return buffer[9+8*len(value):]

		case string:
			buffer[0] = stringid
			binary.BigEndian.PutUint64(buffer[1:], uint64(len(value)))
			copy(buffer[9:], value)
			return buffer[9+len(value):]
		}

		reflection := reflect.ValueOf(item)

		buffer[0] = arrayid
		binary.BigEndian.PutUint64(buffer[1:], uint64(reflection.Len()))

		cursor := buffer[9:]

		for index := 0; index < reflection.Len(); index++ {
			cursor = write(cursor, reflection.Index(index).Interface())
		}

		return cursor
	}

	cursor := write(buffer, item)

	for _, variadicitem := range items {
		cursor = write(cursor, variadicitem)
	}

	return csha256.Sum256(buffer)
}
