// Схема шифрования Эль-Гамаля на эллиптической кривой.

package ecelgamal

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"go-cipher/internal/crypto/ec"
)

const (
	K = 100

	headerBytes = 4
)

var (
	ErrPublicKey = errors.New("ecelgamal: invalid public key")
	ErrPrivateKey = errors.New("ecelgamal: private key out of range")
	ErrCiphertext = errors.New("ecelgamal: malformed ciphertext")
	ErrChunk = errors.New("ecelgamal: message chunk is too long")
	ErrMapping = errors.New("ecelgamal: no curve point maps the chunk")
	ErrUnsupportedPrime = errors.New("ecelgamal: curve prime is not 3 mod 4")
	ErrCurveTooSmall = errors.New("ecelgamal: curve modulus is too small")
	ErrNonce = errors.New("ecelgamal: cannot generate nonce")
)

var (
	one = big.NewInt(1)
	kInt = big.NewInt(K)
)

func GenerateKey(c *ec.Curve, rnd io.Reader) (*big.Int, ec.Point, error) {
	if err := validateCurve(c); err != nil {
		return nil, ec.Point{}, err
	}

	d, err := randomScalar(c.N, rnd)
	if err != nil {
		return nil, ec.Point{}, err
	}
	return d, c.ScalarMult(d, c.Base()), nil
}

func Encrypt(c *ec.Curve, public ec.Point, msg []byte, rnd io.Reader) ([]byte, error) {
	if err := validateCurve(c); err != nil {
		return nil, err
	}
	if err := validatePublicKey(c, public); err != nil {
		return nil, err
	}
	if rnd == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrNonce)
	}
	if uint64(len(msg)) > 1<<32-1 {
		return nil, fmt.Errorf("%w: message is %d bytes", ErrChunk, len(msg))
	}

	width := coordinateWidth(c)
	chunkBytes := ChunkBytes(c)

	payload := make([]byte, headerBytes+len(msg))
	binary.BigEndian.PutUint32(payload[:headerBytes], uint32(len(msg)))
	copy(payload[headerBytes:], msg)

	blocks := (len(payload) + chunkBytes - 1) / chunkBytes
	cipherText := make([]byte, 0, blocks*4*width)

	for i := 0; i < blocks; i++ {
		chunk := make([]byte, chunkBytes)
		copy(chunk, payload[i*chunkBytes:])

		m, err := MessageToPoint(c, chunk)
		if err != nil {
			return nil, err
		}

		k, err := randomScalar(c.N, rnd)
		if err != nil {
			return nil, err
		}
		c1 := c.ScalarMult(k, c.Base())
		c2 := c.Add(m, c.ScalarMult(k, public))

		cipherText = appendPoint(cipherText, c1, width)
		cipherText = appendPoint(cipherText, c2, width)
	}

	return cipherText, nil
}

func Decrypt(c *ec.Curve, priv *big.Int, cipherText []byte) ([]byte, error) {
	if err := validateCurve(c); err != nil {
		return nil, err
	}
	if priv == nil || priv.Sign() <= 0 || priv.Cmp(c.N) >= 0 {
		return nil, ErrPrivateKey
	}

	width := coordinateWidth(c)
	blockBytes := 4 * width
	if len(cipherText) == 0 || len(cipherText)%blockBytes != 0 {
		return nil, fmt.Errorf("%w: length %d is not a multiple of %d", ErrCiphertext, len(cipherText), blockBytes)
	}

	chunkBytes := ChunkBytes(c)
	blocks := len(cipherText) / blockBytes
	payload := make([]byte, 0, blocks*chunkBytes)

	for i := 0; i < blocks; i++ {
		block := cipherText[i*blockBytes : (i+1)*blockBytes]
		c1, c2 := parseBlock(block, width)
		if c1.IsInfinity() || c2.IsInfinity() || !c.IsOnCurve(c1) || !c.IsOnCurve(c2) {
			return nil, fmt.Errorf("%w: block %d is not a pair of curve points", ErrCiphertext, i)
		}

		m := c.Add(c2, c.Neg(c.ScalarMult(priv, c1)))
		if m.IsInfinity() {
			return nil, fmt.Errorf("%w: block %d decrypts to the point at infinity", ErrCiphertext, i)
		}

		chunk, err := PointToMessage(c, m, chunkBytes)
		if err != nil {
			return nil, fmt.Errorf("%w: block %d", err, i)
		}
		payload = append(payload, chunk...)
	}

	if len(payload) < headerBytes {
		return nil, fmt.Errorf("%w: payload is shorter than the header", ErrCiphertext)
	}
	length := binary.BigEndian.Uint32(payload[:headerBytes])
	if uint64(length) > uint64(len(payload)-headerBytes) {
		return nil, fmt.Errorf("%w: declared length %d exceeds %d bytes", ErrCiphertext, length, len(payload)-headerBytes)
	}

	return payload[headerBytes : headerBytes+int(length)], nil
}

func MessageToPoint(c *ec.Curve, chunk []byte) (ec.Point, error) {
	if err := validateCurve(c); err != nil {
		return ec.Point{}, err
	}
	chunkBytes := ChunkBytes(c)
	if len(chunk) > chunkBytes {
		return ec.Point{}, fmt.Errorf("%w: %d bytes, at most %d", ErrChunk, len(chunk), chunkBytes)
	}
	if !primeIs3Mod4(c.P) {
		return ec.Point{}, fmt.Errorf("%w: p mod 4 = %d", ErrUnsupportedPrime, new(big.Int).Mod(c.P, big.NewInt(4)))
	}

	m := new(big.Int).SetBytes(chunk)
	for j := 0; j < K; j++ {
		x := new(big.Int).Mul(m, kInt)
		x.Add(x, big.NewInt(int64(j)))
		if x.Cmp(c.P) >= 0 {
			break
		}

		fx := curveRightHandSide(c, x)
		y, ok := squareRoot(c.P, fx)
		if !ok {
			continue
		}

		p := ec.Point{X: x, Y: y}
		if !c.IsOnCurve(p) {
			continue
		}
		return p, nil
	}

	return ec.Point{}, fmt.Errorf("%w: no j in 0..%d", ErrMapping, K-1)
}

func PointToMessage(c *ec.Curve, p ec.Point, chunkBytes int) ([]byte, error) {
	if err := validateCurve(c); err != nil {
		return nil, err
	}
	if p.IsInfinity() {
		return nil, fmt.Errorf("%w: point at infinity", ErrCiphertext)
	}
	if !c.IsOnCurve(p) {
		return nil, fmt.Errorf("%w: %v", ErrCiphertext, ec.ErrPointNotOnCurve)
	}

	m := new(big.Int).Quo(p.X, kInt)
	if m.BitLen() > 8*chunkBytes {
		return nil, fmt.Errorf("%w: x coordinate %s does not encode %d bytes", ErrMapping, p.X, chunkBytes)
	}
	chunk := m.FillBytes(make([]byte, chunkBytes))

	check, err := MessageToPoint(c, chunk)
	if err != nil {
		return nil, err
	}
	if !check.Equal(p) {
		return nil, fmt.Errorf("%w: x coordinate %s is not a message mapping", ErrMapping, p.X)
	}
	return chunk, nil
}

func ChunkBytes(c *ec.Curve) int {
	if c == nil || c.P == nil {
		return 0
	}
	return (c.P.BitLen() - 8) / 8
}

func validateCurve(c *ec.Curve) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !primeIs3Mod4(c.P) {
		return fmt.Errorf("%w: p mod 4 = %d", ErrUnsupportedPrime, new(big.Int).Mod(c.P, big.NewInt(4)))
	}
	if ChunkBytes(c) < 1 {
		return fmt.Errorf("%w: |p| = %d bits", ErrCurveTooSmall, c.P.BitLen())
	}
	return nil
}

func validatePublicKey(c *ec.Curve, public ec.Point) error {
	if public.IsInfinity() || !c.IsOnCurve(public) {
		return ErrPublicKey
	}
	if !c.ScalarMult(c.N, public).IsInfinity() {
		return fmt.Errorf("%w: point order is not N", ErrPublicKey)
	}
	return nil
}

func primeIs3Mod4(p *big.Int) bool {
	return new(big.Int).And(p, big.NewInt(3)).Cmp(big.NewInt(3)) == 0
}

func curveRightHandSide(c *ec.Curve, x *big.Int) *big.Int {
	fx := new(big.Int).Mul(x, x)
	fx.Mul(fx, x)
	fx.Add(fx, new(big.Int).Mul(c.A, x))
	fx.Add(fx, c.B)
	fx.Mod(fx, c.P)
	return fx
}

func squareRoot(p, f *big.Int) (*big.Int, bool) {
	if f.Sign() == 0 {
		return big.NewInt(0), true
	}

	legendre := new(big.Int).Exp(f, new(big.Int).Rsh(new(big.Int).Sub(p, one), 1), p)
	if legendre.Cmp(one) != 0 {
		return nil, false
	}

	exponent := new(big.Int).Add(p, one)
	exponent.Rsh(exponent, 2)
	y := new(big.Int).Exp(f, exponent, p)

	check := new(big.Int).Mul(y, y)
	check.Mod(check, p)
	if check.Cmp(f) != 0 {
		return nil, false
	}
	return y, true
}

func randomScalar(n *big.Int, rnd io.Reader) (*big.Int, error) {
	for {
		k, err := rand.Int(rnd, n)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNonce, err)
		}
		if k.Sign() != 0 {
			return k, nil
		}
	}
}

func coordinateWidth(c *ec.Curve) int {
	return (c.P.BitLen() + 7) / 8
}

func appendPoint(buf []byte, p ec.Point, width int) []byte {
	start := len(buf)
	buf = append(buf, make([]byte, 2*width)...)
	p.X.FillBytes(buf[start : start+width])
	p.Y.FillBytes(buf[start+width : start+2*width])
	return buf
}

func parseBlock(block []byte, width int) (ec.Point, ec.Point) {
	c1 := ec.Point{
		X: new(big.Int).SetBytes(block[:width]),
		Y: new(big.Int).SetBytes(block[width : 2*width]),
	}
	c2 := ec.Point{
		X: new(big.Int).SetBytes(block[2*width : 3*width]),
		Y: new(big.Int).SetBytes(block[3*width:]),
	}
	return c1, c2
}
