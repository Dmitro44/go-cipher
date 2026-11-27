// Пакет gost3410 реализует формирование и проверку электронной цифровой
// подписи по ГОСТ Р 34.10-2012 (RFC 7091).
//
// Обозначения стандарта: q — порядок базовой точки G, d — закрытый ключ,
// Q = d*G — открытый ключ, e — свёртка хеш-кода сообщения по модулю q,
// k — одноразовый случайный множитель, (r, s) — подпись.
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
	// ErrPrivateKey — закрытый ключ не лежит в диапазоне 1..q-1.
	ErrPrivateKey = errors.New("gost3410: private key out of range")
	// ErrPublicKey — открытый ключ не является точкой кривой порядка q.
	ErrPublicKey = errors.New("gost3410: invalid public key")
	// ErrNonce — не удалось получить случайное k.
	ErrNonce = errors.New("gost3410: cannot generate nonce")
	// ErrHash — не задана хеш-функция.
	ErrHash = errors.New("gost3410: hash function is not set")
)

var (
	one = big.NewInt(1)
)

// GenerateKey формирует ключевую пару: случайный закрытый ключ
// d из диапазона 1..q-1 и открытый ключ Q = d*G.
func GenerateKey(c *ec.Curve) (*big.Int, ec.Point, error) {
	if err := c.Validate(); err != nil {
		return nil, ec.Point{}, err
	}

	// d = random(1..q-1).
	limit := new(big.Int).Sub(c.N, one)
	d, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, ec.Point{}, fmt.Errorf("gost3410: generate private key: %w", err)
	}
	d.Add(d, one)

	return d, c.ScalarMult(d, c.Base()), nil
}

// Sign вычисляет подпись (r, s) сообщения msg.
//
// Шаги алгоритма I стандарта:
//  1. H = h(msg), e = int(H) mod q (при e = 0 берётся e = 1);
//  2. выбирается случайное k из 1..q-1;
//  3. C = k*G, r = x_C mod q; при r = 0 выбирается новое k;
//  4. s = (r*d + k*e) mod q; при s = 0 выбирается новое k.
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

// Verify проверяет подпись (r, s) сообщения msg открытым ключом pub.
//
// Шаги алгоритма II стандарта:
//  1. проверяется 0 < r < q и 0 < s < q;
//  2. e вычисляется так же, как при подписи, v = e^{-1} mod q;
//  3. z1 = s*v mod q, z2 = -r*v mod q;
//  4. C = z1*G + z2*Q; подпись верна, если x_C mod q = r.
//
// Для корректной подписи возвращается true. Математически неверная
// подпись, в том числе с r или s вне диапазона, даёт false без ошибки:
// это штатный результат проверки, а не сбой.
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

	// v = e^{-1} mod q.
	v := new(big.Int).ModInverse(e, c.N)
	if v == nil {
		return false, fmt.Errorf("gost3410: e has no inverse modulo q")
	}

	// z1 = s*v mod q, z2 = -r*v mod q.
	z1 := new(big.Int).Mul(s, v)
	z1.Mod(z1, c.N)

	z2 := new(big.Int).Mul(r, v)
	z2.Neg(z2)
	z2.Mod(z2, c.N)

	// C = z1*G + z2*Q.
	cPoint := c.Add(c.ScalarMult(z1, c.Base()), c.ScalarMult(z2, pub))
	if cPoint.IsInfinity() {
		return false, nil
	}

	// R = x_C mod q.
	bigR := new(big.Int).Mod(cPoint.X, c.N)
	return bigR.Cmp(r) == 0, nil
}

// signWithNonce выполняет шаги 3-4 алгоритма I для уже выбранных e и k.
// ok = false означает, что получено r = 0 или s = 0 и нужно новое k.
func signWithNonce(c *ec.Curve, priv, e, k *big.Int) (r, s *big.Int, ok bool) {
	cPoint := c.ScalarMult(k, c.Base())
	if cPoint.IsInfinity() {
		return nil, nil, false
	}

	r = new(big.Int).Mod(cPoint.X, c.N)
	if r.Sign() == 0 {
		return nil, nil, false
	}

	// s = (r*d + k*e) mod q.
	s = new(big.Int).Mul(r, priv)
	s.Add(s, new(big.Int).Mul(k, e))
	s.Mod(s, c.N)
	if s.Sign() == 0 {
		return nil, nil, false
	}

	return r, s, true
}

// hashToE вычисляет e = int(h(msg)) mod q, заменяя нулевой результат на 1.
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

// randomK выбирает k из диапазона 1..q-1, читая байты из rnd.
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
