package cipher

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"math/big"

	"go-cipher/internal/crypto/ec"
	"go-cipher/internal/crypto/gost3410"
	gosthash "go-cipher/internal/crypto/hash"
)

var (
	// ErrUnsupportedKeySize — длина ключа или подписи не соответствует ни
	// одному набору параметров.
	ErrUnsupportedKeySize = errors.New("gost3410: unsupported key or signature size")
)

// Gost3410 — адаптер ГОСТ Р 34.10-2012 к интерфейсу SignatureScheme.
//
// Ключи и подпись кодируются числами фиксированной ширины, равной длине
// порядка q в байтах: 32 байта для 256-битной кривой и 64 байта для
// 512-битной. Открытый ключ — x||y, закрытый — d, подпись — r||s.
type Gost3410 struct{}

func NewGost3410() Gost3410 {
	return Gost3410{}
}

func (Gost3410) Name() string {
	return "ГОСТ Р 34.10-2012"
}

func (Gost3410) KeyOptions() []KeyOption {
	return []KeyOption{
		{Label: "256-bit curve", Bits: 256},
		{Label: "512-bit curve", Bits: 512},
	}
}

func (Gost3410) GenerateKeys(option KeyOption) (KeyPair, error) {
	c, err := curveByBits(option.Bits)
	if err != nil {
		return KeyPair{}, err
	}

	priv, pub, err := gost3410.GenerateKey(c)
	if err != nil {
		return KeyPair{}, err
	}

	width := numberWidth(c)
	// Открытый ключ — x||y, закрытый — d.
	public := make([]byte, 2*width)
	pub.X.FillBytes(public[:width])
	pub.Y.FillBytes(public[width:])

	private := make([]byte, width)
	priv.FillBytes(private)

	return KeyPair{Public: public, Private: private}, nil
}

func (Gost3410) Sign(priv, msg []byte) ([]byte, error) {
	c, err := curveByPrivate(priv)
	if err != nil {
		return nil, err
	}

	d := new(big.Int).SetBytes(priv)
	r, s, err := gost3410.Sign(c, hashFor(c), d, msg, rand.Reader)
	if err != nil {
		return nil, err
	}

	width := numberWidth(c)
	sig := make([]byte, 2*width)
	r.FillBytes(sig[:width])
	s.FillBytes(sig[width:])
	return sig, nil
}

func (Gost3410) Verify(pub, msg, sig []byte) (bool, error) {
	c, err := curveByPublic(pub)
	if err != nil {
		return false, err
	}

	width := numberWidth(c)
	if len(sig) != 2*width {
		return false, fmt.Errorf("%w: signature is %d bytes, expected %d", ErrUnsupportedKeySize, len(sig), 2*width)
	}

	q := ec.Point{
		X: new(big.Int).SetBytes(pub[:width]),
		Y: new(big.Int).SetBytes(pub[width:]),
	}
	r := new(big.Int).SetBytes(sig[:width])
	s := new(big.Int).SetBytes(sig[width:])

	return gost3410.Verify(c, hashFor(c), q, msg, r, s)
}

// curveByBits выбирает кривую по размеру ключа в битах.
func curveByBits(bits int) (*ec.Curve, error) {
	switch bits {
	case 256:
		return ec.Params256(), nil
	case 512:
		return ec.Params512(), nil
	default:
		return nil, fmt.Errorf("%w: %d bits", ErrUnsupportedKeySize, bits)
	}
}

// curveByPrivate определяет кривую по длине закрытого ключа: 32 байта —
// 256-битная, 64 байта — 512-битная.
func curveByPrivate(priv []byte) (*ec.Curve, error) {
	switch len(priv) {
	case 32:
		return ec.Params256(), nil
	case 64:
		return ec.Params512(), nil
	default:
		return nil, fmt.Errorf("%w: private key is %d bytes", ErrUnsupportedKeySize, len(priv))
	}
}

// curveByPublic определяет кривую по длине открытого ключа x||y: 64 байта —
// 256-битная, 128 байт — 512-битная.
func curveByPublic(pub []byte) (*ec.Curve, error) {
	switch len(pub) {
	case 64:
		return ec.Params256(), nil
	case 128:
		return ec.Params512(), nil
	default:
		return nil, fmt.Errorf("%w: public key is %d bytes", ErrUnsupportedKeySize, len(pub))
	}
}

// numberWidth возвращает ширину числа подписи в байтах.
func numberWidth(c *ec.Curve) int {
	return (c.N.BitLen() + 7) / 8
}

// hashFor выбирает хеш-функцию по размеру кривой: 256-битной кривой
// соответствует ГОСТ Р 34.11-2012 с длиной кода 256 бит, 512-битной — 512.
func hashFor(c *ec.Curve) func() hash.Hash {
	if numberWidth(c) == 32 {
		return gosthash.NewGOST3411_256
	}
	return gosthash.NewGOST3411_512
}
