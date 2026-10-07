package gost

import (
	"encoding/binary"
	"math/bits"
)

const (
	KeySize = 32
)

// id-tc26-gost-28147-param-Z (RFC 7836)
var sbox = [8][16]uint8{
	{0xc, 0x4, 0x6, 0x2, 0xa, 0x5, 0xb, 0x9, 0xe, 0x8, 0xd, 0x7, 0x0, 0x3, 0xf, 0x1},
	{0x6, 0x8, 0x2, 0x3, 0x9, 0xa, 0x5, 0xc, 0x1, 0xe, 0x4, 0x7, 0xb, 0xd, 0x0, 0xf},
	{0xb, 0x3, 0x5, 0x8, 0x2, 0xf, 0xa, 0xd, 0xe, 0x1, 0x7, 0x4, 0xc, 0x9, 0x6, 0x0},
	{0xc, 0x8, 0x2, 0x1, 0xd, 0x4, 0xf, 0x6, 0x7, 0x0, 0xa, 0x5, 0x3, 0xe, 0x9, 0xb},
	{0x7, 0xf, 0x5, 0xa, 0x8, 0x1, 0x6, 0xd, 0x0, 0x9, 0x3, 0xe, 0xb, 0x4, 0x2, 0xc},
	{0x5, 0xd, 0xf, 0x6, 0x9, 0x2, 0xc, 0xa, 0xb, 0x7, 0x8, 0x1, 0x4, 0x3, 0xe, 0x0},
	{0x8, 0xe, 0x2, 0x5, 0x6, 0x9, 0x1, 0xc, 0xf, 0x4, 0xb, 0x0, 0xd, 0xa, 0x3, 0x7},
	{0x1, 0x7, 0xe, 0xd, 0x0, 0x5, 0x8, 0x3, 0x4, 0xf, 0xa, 0x6, 0x9, 0xc, 0xb, 0x2},
}

func KeysForRoundEncrypt(key []byte, rounds int) []uint32 {
	subkeys := make([]uint32, 8)
	for i := range 8 {
		subkeys[i] = binary.LittleEndian.Uint32(key[i*4 : i*4+4])
	}

	result := make([]uint32, rounds)
	for i := range rounds {
		if i < 24 {
			result[i] = subkeys[i%8]
		} else {
			result[i] = subkeys[7-i%8]
		}
	}

	return result
}

func KeysForRoundDecrypt(key []byte, rounds int) []uint32 {
	subkeys := make([]uint32, 8)
	for i := range 8 {
		subkeys[i] = binary.LittleEndian.Uint32(key[i*4 : i*4+4])
	}

	result := make([]uint32, rounds)
	for i := range rounds {
		if i < 8 {
			result[i] = subkeys[i]
		} else {
			result[i] = subkeys[7-i%8]
		}
	}

	return result
}

func f(left uint32, subkey uint32) uint32 {
	s := left + subkey

	var result uint32
	for i := range 8 {
		// 4 bits
		x := uint8((s >> (4 * i)) & 0xF)
		replaced := sbox[i][x]

		result |= uint32(replaced) << (4 * i)
	}

	result = bits.RotateLeft32(result, 11)
	return result
}

func BlockEncryptParts(input []byte, key []byte, rounds int) (uint32, uint32) {
	left := binary.LittleEndian.Uint32(input[0:4])
	right := binary.LittleEndian.Uint32(input[4:8])

	subkeys := KeysForRoundEncrypt(key, rounds)

	for i := range rounds {
		left, right = right^f(left, subkeys[i]), left
	}

	return right, left
}

func BlockDecryptParts(input []byte, key []byte, rounds int) (uint32, uint32) {
	left := binary.LittleEndian.Uint32(input[0:4])
	right := binary.LittleEndian.Uint32(input[4:8])

	subkeys := KeysForRoundDecrypt(key, rounds)

	for i := range rounds {
		left, right = right^f(left, subkeys[i]), left
	}

	return right, left
}

func BlockEncrypt32(input []byte, key []byte) []byte {
	left, right := BlockEncryptParts(input, key, 32)

	combined := make([]byte, 8)
	binary.LittleEndian.PutUint32(combined[0:4], left)
	binary.LittleEndian.PutUint32(combined[4:8], right)
	return combined
}

func BlockDecrypt32(input []byte, key []byte) []byte {
	left, right := BlockDecryptParts(input, key, 32)

	combined := make([]byte, 8)
	binary.LittleEndian.PutUint32(combined[0:4], left)
	binary.LittleEndian.PutUint32(combined[4:8], right)
	return combined
}
