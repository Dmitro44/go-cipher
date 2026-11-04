package cipher

import (
	"crypto/rand"
	"math/big"

	"go-cipher/internal/crypto/ec"
	"go-cipher/internal/crypto/ecelgamal"
)

// ElGamal — адаптер схемы Эль-Гамаля на эллиптических кривых к интерфейсу
// AsymCipher.
//
// Ключи кодируются числами фиксированной ширины, равной длине модуля p в
// байтах: 32 байта для 256-битной кривой и 64 для 512-битной. Открытый ключ
// — x||y точки Q, закрытый — само число d. Шифртекст — последовательность
// блоков C1||C2, которые формирует пакет ecelgamal.
type ElGamal struct{}

func NewElGamal() ElGamal {
	return ElGamal{}
}

func (ElGamal) Name() string {
	return "EC ElGamal"
}

func (ElGamal) KeyOptions() []KeyOption {
	return []KeyOption{
		{Label: "256-bit curve", Bits: 256},
		{Label: "512-bit curve", Bits: 512},
	}
}

func (ElGamal) GenerateKeys(option KeyOption) (KeyPair, error) {
	c, err := curveByBits(option.Bits)
	if err != nil {
		return KeyPair{}, err
	}

	priv, pub, err := ecelgamal.GenerateKey(c, rand.Reader)
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

func (ElGamal) Encrypt(pub, msg []byte) ([]byte, error) {
	c, err := curveByPublic(pub)
	if err != nil {
		return nil, err
	}

	width := numberWidth(c)
	q := ec.Point{
		X: new(big.Int).SetBytes(pub[:width]),
		Y: new(big.Int).SetBytes(pub[width:]),
	}

	return ecelgamal.Encrypt(c, q, msg, rand.Reader)
}

func (ElGamal) Decrypt(priv, cipherText []byte) ([]byte, error) {
	c, err := curveByPrivate(priv)
	if err != nil {
		return nil, err
	}

	d := new(big.Int).SetBytes(priv)
	return ecelgamal.Decrypt(c, d, cipherText)
}
