// Подпись ГОСТ Р 34.10-2012: ключи, формирование и проверка.

package gost3410

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"

	"go-cipher/internal/crypto/ec"
)

var (
	ErrPrivateKey = errors.New("gost3410: private key out of range")
	ErrPublicKey = errors.New("gost3410: invalid public key")
	ErrNonce = errors.New("gost3410: cannot generate nonce")
	ErrHash = errors.New("gost3410: hash function is not set")
)

var (
	one = big.NewInt(1)
)

func GenerateKey(c *ec.Curve) (*big.Int, ec.Point, error) {
	if err := c.Validate(); err != nil {
		return nil, ec.Point{}, err
	}

	limit := new(big.Int).Sub(c.N, one)
	d, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, ec.Point{}, fmt.Errorf("gost3410: generate private key: %w", err)
	}
	d.Add(d, one)

	return d, c.ScalarMult(d, c.Base()), nil
}

func Sign(c *ec.Curve, hashAlg func() hash.Hash, priv *big.Int, msg []byte, rnd io.Reader) (*big.Int, *big.Int, error) {
	if c == nil || c.N == nil || c.N.Cmp(one) <= 0 {
		return nil, nil, ec.ErrInvalidParams
	}
	if priv == nil || priv.Sign() <= 0 || priv.Cmp(c.N) >= 0 {
		return nil, nil, ErrPrivateKey
	}

	e, err := hashToE(c, hashAlg, msg)
	if err != nil {
		return nil, nil, err
	}

	for {
		k, err := randomK(c.N, rnd)
		if err != nil {
			return nil, nil, err
		}

		r, s, ok := signWithNonce(c, priv, e, k)
		if ok {
			return r, s, nil
		}
	}
}

func Verify(c *ec.Curve, hashAlg func() hash.Hash, pub ec.Point, msg []byte, r, s *big.Int) (bool, error) {
	if c == nil || c.N == nil || c.N.Cmp(one) <= 0 {
		return false, ec.ErrInvalidParams
	}
	if pub.IsInfinity() || !c.IsOnCurve(pub) || !c.ScalarMult(c.N, pub).IsInfinity() {
		return false, ErrPublicKey
	}
	if r == nil || s == nil || r.Sign() <= 0 || s.Sign() <= 0 || r.Cmp(c.N) >= 0 || s.Cmp(c.N) >= 0 {
		return false, nil
	}

	e, err := hashToE(c, hashAlg, msg)
	if err != nil {
		return false, err
	}

	v := new(big.Int).ModInverse(e, c.N)
	if v == nil {
		return false, fmt.Errorf("gost3410: e has no inverse modulo q")
	}

	z1 := new(big.Int).Mul(s, v)
	z1.Mod(z1, c.N)

	z2 := new(big.Int).Mul(r, v)
	z2.Neg(z2)
	z2.Mod(z2, c.N)

	cPoint := c.Add(c.ScalarMult(z1, c.Base()), c.ScalarMult(z2, pub))
	if cPoint.IsInfinity() {
		return false, nil
	}

	bigR := new(big.Int).Mod(cPoint.X, c.N)
	return bigR.Cmp(r) == 0, nil
}

func signWithNonce(c *ec.Curve, priv, e, k *big.Int) (r, s *big.Int, ok bool) {
	cPoint := c.ScalarMult(k, c.Base())
	if cPoint.IsInfinity() {
		return nil, nil, false
	}

	r = new(big.Int).Mod(cPoint.X, c.N)
	if r.Sign() == 0 {
		return nil, nil, false
	}

	s = new(big.Int).Mul(r, priv)
	s.Add(s, new(big.Int).Mul(k, e))
	s.Mod(s, c.N)
	if s.Sign() == 0 {
		return nil, nil, false
	}

	return r, s, true
}

func hashToE(c *ec.Curve, hashAlg func() hash.Hash, msg []byte) (*big.Int, error) {
	if hashAlg == nil {
		return nil, ErrHash
	}

	h := hashAlg()
	h.Write(msg)
	e := new(big.Int).SetBytes(h.Sum(nil))
	e.Mod(e, c.N)
	if e.Sign() == 0 {
		e.SetInt64(1)
	}
	return e, nil
}

func randomK(q *big.Int, rnd io.Reader) (*big.Int, error) {
	for {
		k, err := rand.Int(rnd, q)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNonce, err)
		}
		if k.Sign() != 0 {
			return k, nil
		}
	}
}
