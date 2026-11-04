package cipher

import (
	"errors"
	"math/big"

	"go-cipher/internal/crypto/rabin"
)

type Rabin struct{}

func NewRabin() Rabin {
	return Rabin{}
}

func (Rabin) Name() string {
	return "Rabin"
}

func (Rabin) KeyOptions() []KeyOption {
	return []KeyOption{
		{Label: "512-bit modulus", Bits: 512},
		{Label: "1024-bit modulus", Bits: 1024},
		{Label: "2048-bit modulus", Bits: 2048},
	}
}

// Публичный ключ — n, приватный — p||q с фиксированной шириной каждого простого.
func (Rabin) GenerateKeys(option KeyOption) (KeyPair, error) {
	p, q, err := rabin.GenerateKeys(option.Bits)
	if err != nil {
		return KeyPair{}, err
	}
	n := new(big.Int).Mul(p, q)
	primeBytes := option.Bits / 16
	return KeyPair{
		Public:  n.FillBytes(make([]byte, option.Bits/8)),
		Private: append(p.FillBytes(make([]byte, primeBytes)), q.FillBytes(make([]byte, primeBytes))...),
	}, nil
}

func (Rabin) Encrypt(pub, msg []byte) ([]byte, error) {
	return rabin.Encrypt(new(big.Int).SetBytes(pub), msg)
}

func (Rabin) Decrypt(priv, cipherText []byte) ([][]byte, error) {
	if len(priv) == 0 || len(priv)%2 != 0 {
		return nil, errors.New("private key must be p||q of equal length")
	}
	half := len(priv) / 2
	p := new(big.Int).SetBytes(priv[:half])
	q := new(big.Int).SetBytes(priv[half:])

	plain, err := rabin.Decrypt(p, q, cipherText)
	if err != nil {
		return nil, err
	}
	return [][]byte{plain}, nil
}
