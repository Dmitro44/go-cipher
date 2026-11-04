package rabin

import (
	"bytes"
	"crypto/rand"
	"errors"
	"math/big"
)

var tag = []byte("MZIRABIN")

// Маркер конца данных: без него завершающие нулевые байты сообщения неотличимы от паддинга.
const padMarker = 0x80

var (
	one = big.NewInt(1)
	two = big.NewInt(2)

	ErrKeyTooSmall = errors.New("modulus too small for redundancy block")
	ErrInvalidKey  = errors.New("private key must be two primes congruent to 3 mod 4")
	ErrBadCipher   = errors.New("ciphertext length or values do not match the key")
	ErrNoValidRoot = errors.New("no square root carries valid redundancy: wrong key or corrupted ciphertext")
)

func GenerateKeys(bits int) (*big.Int, *big.Int, error) {
	p, err := randPrime3mod4(bits / 2)
	if err != nil {
		return nil, nil, err
	}
	for {
		q, err := randPrime3mod4(bits / 2)
		if err != nil {
			return nil, nil, err
		}
		if q.Cmp(p) != 0 {
			return p, q, nil
		}
	}
}

func randPrime3mod4(bits int) (*big.Int, error) {
	for {
		p, err := rand.Prime(rand.Reader, bits)
		if err != nil {
			return nil, err
		}
		if p.Bit(1) == 1 {
			return p, nil
		}
	}
}

func Encrypt(n *big.Int, msg []byte) ([]byte, error) {
	size, err := payloadLen(n)
	if err != nil {
		return nil, err
	}
	k := byteLen(n)
	pieces := chunk(msg, size)
	out := make([]byte, len(pieces)*k)

	// Блок и big.Int переиспользуются между блоками: на блок не приходится ни одной аллокации.
	block := make([]byte, blockLen(n))
	m, c := new(big.Int), new(big.Int)
	for i, payload := range pieces {
		m.SetBytes(padInto(block, payload))
		c.Exp(m, two, n)
		c.FillBytes(out[i*k : (i+1)*k])
	}
	return out, nil
}

func Decrypt(p, q *big.Int, ct []byte) ([]byte, error) {
	if !validPrivate(p, q) {
		return nil, ErrInvalidKey
	}
	n := new(big.Int).Mul(p, q)
	if _, err := payloadLen(n); err != nil {
		return nil, err
	}
	k := byteLen(n)
	if len(ct) == 0 || len(ct)%k != 0 {
		return nil, ErrBadCipher
	}

	var plain []byte
	c := new(big.Int)
	for off := 0; off < len(ct); off += k {
		c.SetBytes(ct[off : off+k])
		if c.Cmp(n) >= 0 {
			return nil, ErrBadCipher
		}
		payload, err := recoverPayload(Roots(c, p, q), blockLen(n))
		if err != nil {
			return nil, err
		}
		plain = append(plain, payload...)
	}
	return plain, nil
}

func Roots(c, p, q *big.Int) []*big.Int {
	mp := sqrtModPrime(c, p)
	mq := sqrtModPrime(c, q)

	// yp*p + yq*q = 1: eq = yq*q ≡ 1 (mod p), ep = yp*p ≡ 1 (mod q) — коэффициенты CRT.
	yp, yq := new(big.Int), new(big.Int)
	new(big.Int).GCD(yp, yq, p, q)
	n := new(big.Int).Mul(p, q)
	ep := new(big.Int).Mul(yp, p)
	eq := new(big.Int).Mul(yq, q)

	r := new(big.Int).Mul(mp, eq)
	r.Add(r, new(big.Int).Mul(mq, ep))
	r.Mod(r, n)

	s := new(big.Int).Mul(mp, eq)
	s.Sub(s, new(big.Int).Mul(mq, ep))
	s.Mod(s, n)

	return []*big.Int{r, negMod(r, n), s, negMod(s, n)}
}

func sqrtModPrime(c, p *big.Int) *big.Int {
	e := new(big.Int).Add(p, one)
	e.Rsh(e, 2)
	return new(big.Int).Exp(c, e, p)
}

func negMod(x, n *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Neg(x), n)
}

func validPrivate(p, q *big.Int) bool {
	return p.Bit(1) == 1 && q.Bit(1) == 1 && p.Cmp(q) != 0
}

func byteLen(n *big.Int) int {
	return (n.BitLen() + 7) / 8
}

// Блок на один байт короче n, чтобы любой блок был меньше n.
func blockLen(n *big.Int) int {
	return byteLen(n) - 1
}

func payloadLen(n *big.Int) (int, error) {
	size := blockLen(n) - len(tag) - 1
	if size < 1 {
		return 0, ErrKeyTooSmall
	}
	return size, nil
}

// Всегда возвращает хотя бы один кусок: пустое сообщение тоже даёт блок.
func chunk(msg []byte, size int) [][]byte {
	var pieces [][]byte
	for len(msg) > size {
		pieces = append(pieces, msg[:size])
		msg = msg[size:]
	}
	return append(pieces, msg)
}

func pad(payload []byte, width int) []byte {
	return padInto(make([]byte, width), payload)
}

// padInto заполняет готовый блок целиком, чтобы буфер можно было переиспользовать.
func padInto(block, payload []byte) []byte {
	clear(block) // иначе хвост предыдущего блока останется между маркером и tag
	copy(block, payload)
	block[len(payload)] = padMarker
	copy(block[len(block)-len(tag):], tag)
	return block
}

func recoverPayload(roots []*big.Int, width int) ([]byte, error) {
	for _, r := range roots {
		if payload, ok := blockPayload(r, width); ok {
			return payload, nil
		}
	}
	return nil, ErrNoValidRoot
}

func blockPayload(r *big.Int, width int) ([]byte, bool) {
	if r.BitLen() > 8*width {
		return nil, false
	}
	return unpad(r.FillBytes(make([]byte, width)))
}

func unpad(block []byte) ([]byte, bool) {
	if !bytes.HasSuffix(block, tag) {
		return nil, false
	}
	body := bytes.TrimRight(block[:len(block)-len(tag)], "\x00")
	if len(body) == 0 || body[len(body)-1] != padMarker {
		return nil, false
	}
	return body[:len(body)-1], true
}
