package hash

// Хеш-функция SHA-1 по FIPS 180-4.

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

const (
	sha1BlockSize = 64
	sha1Size      = 20
)

// Четыре раундовые константы K_t из FIPS 180-4, п. 4.2.1.
var sha1Constants = [4]uint32{
	0x5a827999,
	0x6ed9eba1,
	0x8f1bbcdc,
	0xca62c1d6,
}

// sha1Digest — реализация hash.Hash для SHA-1.
type sha1Digest struct {
	state [5]uint32
	block [sha1BlockSize]byte
	used  int
	total uint64
}

// NewSHA1 возвращает хеш-функцию SHA-1.
func NewSHA1() hash.Hash {
	d := &sha1Digest{}
	d.Reset()
	return d
}

func (d *sha1Digest) Size() int      { return sha1Size }
func (d *sha1Digest) BlockSize() int { return sha1BlockSize }

// Reset возвращает буфер в начальное значение IV из FIPS 180-4, п. 5.3.1.
func (d *sha1Digest) Reset() {
	d.state = [5]uint32{
		0x67452301,
		0xefcdab89,
		0x98badcfe,
		0x10325476,
		0xc3d2e1f0,
	}
	d.used = 0
	d.total = 0
}

// Write добавляет данные, обрабатывая все накопленные полные блоки.
func (d *sha1Digest) Write(p []byte) (int, error) {
	n := len(p)
	d.total += uint64(n)

	for len(p) > 0 {
		copied := copy(d.block[d.used:], p)
		d.used += copied
		p = p[copied:]
		if d.used == sha1BlockSize {
			d.processBlock(d.block[:])
			d.used = 0
		}
	}
	return n, nil
}

// Sum дописывает дайджест к b, не изменяя состояние.
func (d *sha1Digest) Sum(b []byte) []byte {
	clone := *d
	clone.finish()
	var out [sha1Size]byte
	for i, v := range clone.state {
		binary.BigEndian.PutUint32(out[i*4:], v)
	}
	return append(b, out[:]...)
}

// finish дополняет сообщение по FIPS 180-4, п. 5.1.1: бит 1, нули и
// 64-битная длина сообщения в битах, после чего обрабатывает последний блок.
func (d *sha1Digest) finish() {
	bitLen := d.total * 8

	d.block[d.used] = 0x80
	d.used++

	if d.used > sha1BlockSize-8 {
		for d.used < sha1BlockSize {
			d.block[d.used] = 0
			d.used++
		}
		d.processBlock(d.block[:])
		d.used = 0
	}

	for d.used < sha1BlockSize-8 {
		d.block[d.used] = 0
		d.used++
	}
	binary.BigEndian.PutUint64(d.block[sha1BlockSize-8:], bitLen)
	d.processBlock(d.block[:])
}

// processBlock выполняет 80 раундов SHA-1 для одного 512-битного блока.
func (d *sha1Digest) processBlock(block []byte) {
	var w [80]uint32

	// Первые 16 слов берутся из блока напрямую.
	for i := 0; i < 16; i++ {
		w[i] = binary.BigEndian.Uint32(block[i*4 : i*4+4])
	}
	// Остальные: W[t] = rotl(W[t-16] xor W[t-14] xor W[t-8] xor W[t-3], 1).
	for t := 16; t < 80; t++ {
		w[t] = bits.RotateLeft32(w[t-16]^w[t-14]^w[t-8]^w[t-3], 1)
	}

	a, b, c, e, f := d.state[0], d.state[1], d.state[2], d.state[3], d.state[4]

	for t := 0; t < 80; t++ {
		var fn, k uint32
		switch {
		case t < 20:
			fn = (b & c) | (^b & e)
			k = sha1Constants[0]
		case t < 40:
			fn = b ^ c ^ e
			k = sha1Constants[1]
		case t < 60:
			fn = (b & c) | (b & e) | (c & e)
			k = sha1Constants[2]
		default:
			fn = b ^ c ^ e
			k = sha1Constants[3]
		}

		temp := bits.RotateLeft32(a, 5) + fn + f + k + w[t]
		f = e
		e = c
		c = bits.RotateLeft32(b, 30)
		b = a
		a = temp
	}

	d.state[0] += a
	d.state[1] += b
	d.state[2] += c
	d.state[3] += e
	d.state[4] += f
}
