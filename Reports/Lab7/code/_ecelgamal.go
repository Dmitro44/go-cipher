// Пакет ecelgamal реализует схему шифрования Эль-Гамаля на группе точек
// эллиптической кривой над простым полем.
//
// Обозначения: G — базовая точка порядка N, d — закрытый ключ, Q = d*G —
// открытый ключ, k — одноразовый случайный множитель. Шифртекст каждого
// блока — пара точек (C1, C2) = (k*G, M + k*Q), где M — точка, в которую
// отображён очередной фрагмент сообщения методом Коблица.
//
// Арифметика кривой берётся из пакета ec; здесь нет ни одной операции над
// точками, кроме композиции готовых функций Add, Neg и ScalarMult.
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
	// K — параметр отображения Коблица: байтовому блоку m ищется точка с
	// абсциссой x = m*K + j, j = 0..K-1. Для случайного x кривая проходит
	// примерно через половину значений абсцисс, поэтому подходящее j
	// находится за ~2 попытки, а вероятность неудачи после K попыток не
	// превосходит 2^-K.
	K = 100

	// headerBytes — длина поля с размером сообщения в начале полезной
	// нагрузки. Размер записывается как uint32, старшим байтом вперёд.
	headerBytes = 4
)

var (
	// ErrPublicKey — открытый ключ не является точкой кривой порядка N.
	ErrPublicKey = errors.New("ecelgamal: invalid public key")
	// ErrPrivateKey — закрытый ключ не лежит в диапазоне 1..N-1.
	ErrPrivateKey = errors.New("ecelgamal: private key out of range")
	// ErrCiphertext — шифртекст неверной длины, содержит точку не с
	// кривой или бесконечно удалённую точку.
	ErrCiphertext = errors.New("ecelgamal: malformed ciphertext")
	// ErrChunk — фрагмент сообщения длиннее, чем допускает размер поля.
	ErrChunk = errors.New("ecelgamal: message chunk is too long")
	// ErrMapping — для фрагмента не нашлось точки кривой (отображение
	// Коблица не удалось) либо точка не соответствует никакому фрагменту.
	ErrMapping = errors.New("ecelgamal: no curve point maps the chunk")
	// ErrUnsupportedPrime — модуль кривой не сравним с 3 по модулю 4,
	// поэтому простой формулы квадратного корня (p+1)/4 недостаточно.
	ErrUnsupportedPrime = errors.New("ecelgamal: curve prime is not 3 mod 4")
	// ErrNonce — не удалось получить случайное k.
	ErrNonce = errors.New("ecelgamal: cannot generate nonce")
)

var (
	one = big.NewInt(1)
	// kInt — параметр K в виде big.Int, чтобы не создавать его на каждом
	// шаге отображения.
	kInt = big.NewInt(K)
)

// GenerateKey формирует ключевую пару: случайный закрытый ключ d из
// диапазона 1..N-1 и открытый ключ Q = d*G.
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

// Encrypt зашифровывает сообщение msg открытым ключом public.
//
// Сообщение обрамляется заголовком: uint32 с длиной сообщения в байтах,
// затем сами байты. Обрамлённая строка дополняется нулями до целого числа
// фрагментов по ChunkBytes(c) байт. Каждый фрагмент отображается в точку
// M, после чего для случайного k из 1..N-1 вычисляются
//
//	C1 = k*G,  C2 = M + k*public,
//
// и в шифртекст дописываются четыре координаты C1 и C2 фиксированной
// ширины, по |p| байт каждая.
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

	// Обрамление: длина сообщения, затем его байты, затем нулевое
	// дополнение до целого числа фрагментов. Заголовок входит в полезную
	// нагрузку, поэтому блоков всегда хотя бы один даже для пустого
	// сообщения — иначе расшифрование не смогло бы отличить пустое
	// сообщение от отсутствия данных.
	payload := make([]byte, headerBytes+len(msg))
	binary.BigEndian.PutUint32(payload[:headerBytes], uint32(len(msg)))
	copy(payload[headerBytes:], msg)

	blocks := (len(payload) + chunkBytes - 1) / chunkBytes
	cipherText := make([]byte, 0, blocks*4*width)

	for i := 0; i < blocks; i++ {
		chunk := make([]byte, chunkBytes)
		copy(chunk, payload[i*chunkBytes:])

		// M — точка, изображающая фрагмент.
		m, err := MessageToPoint(c, chunk)
		if err != nil {
			return nil, err
		}

		// k — одноразовый множитель; C1 = k*G, C2 = M + k*Q.
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

// Decrypt расшифровывает шифртекст ct закрытым ключом priv.
//
// Для каждого блока восстанавливается точка
//
//	M = C2 - d*C1 = M + k*Q - d*k*G = M,
//
// поскольку Q = d*G. Из абсциссы M извлекается номер фрагмента m = x/K,
// после чего отбрасывается нулевое дополнение по записанной в заголовке
// длине сообщения.
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

		// M = C2 - d*C1.
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

// MessageToPoint отображает фрагмент сообщения в точку кривой методом
// Коблица. Байты фрагмента интерпретируются как число m; перебираются
// абсциссы x = m*K + j, j = 0..K-1, и берётся первая, для которой
// правая часть уравнения кривой
//
//	f(x) = x^3 + a*x + b (mod p)
//
// является квадратичным вычетом. Ордината вычисляется по формуле
//
//	y = f(x)^((p+1)/4) mod p,
//
// которая верна для простого p = 3 (mod 4) и даёт один из двух корней.
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
		// x = m*K + j; при выбранном размере фрагмента x < p гарантировано.
		x := new(big.Int).Mul(m, kInt)
		x.Add(x, big.NewInt(int64(j)))
		if x.Cmp(c.P) >= 0 {
			break
		}

		// f(x) = x^3 + a*x + b (mod p).
		fx := curveRightHandSide(c, x)
		y, ok := squareRoot(c.P, fx)
		if !ok {
			continue
		}

		p := ec.Point{X: x, Y: y}
		// Уравнению удовлетворяет и точка с ординатой p - y; берётся
		// корень, который возвращает squareRoot, чтобы отображение было
		// однозначным и обратное преобразование могло проверить результат
		// повторным отображением.
		if !c.IsOnCurve(p) {
			continue
		}
		return p, nil
	}

	return ec.Point{}, fmt.Errorf("%w: no j in 0..%d", ErrMapping, K-1)
}

// PointToMessage восстанавливает фрагмент сообщения из точки M: абсцисса
// имеет вид x = m*K + j, поэтому m = x/K. Результат проверяется повторным
// отображением: принимается только та точка, которую MessageToPoint
// порождает для полученного фрагмента. Это отвергает постороннюю точку,
// возникшую при неверном ключе или повреждённом шифртексте.
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

	// m = x / K, остаток j = x mod K должен лежать в 0..K-1, что верно по
	// определению деления с остатком.
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

// ChunkBytes возвращает длину фрагмента сообщения в байтах:
//
//	chunkBytes = (|p| - 8) / 8,
//
// где |p| — битовая длина модуля. Запас в восемь бит нужен для множителя K:
// m < 2^(8*chunkBytes) = 2^(|p|-8), поэтому
//
//	m*K + j < K*2^(|p|-8) = (K/128)*2^(|p|-1) <= p,
//
// так как у p ровно |p| бит, то есть p >= 2^(|p|-1), а K = 100 < 128.
func ChunkBytes(c *ec.Curve) int {
	if c == nil || c.P == nil {
		return 0
	}
	return (c.P.BitLen() - 8) / 8
}

// validateCurve проверяет параметры кривой и пригодность простой формулы
// квадратного корня.
func validateCurve(c *ec.Curve) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if !primeIs3Mod4(c.P) {
		return fmt.Errorf("%w: p mod 4 = %d", ErrUnsupportedPrime, new(big.Int).Mod(c.P, big.NewInt(4)))
	}
	return nil
}

// validatePublicKey проверяет, что открытый ключ — точка кривой порядка N.
func validatePublicKey(c *ec.Curve, public ec.Point) error {
	if public.IsInfinity() || !c.IsOnCurve(public) {
		return ErrPublicKey
	}
	if !c.ScalarMult(c.N, public).IsInfinity() {
		return fmt.Errorf("%w: point order is not N", ErrPublicKey)
	}
	return nil
}

// primeIs3Mod4 сообщает, сравним ли модуль с 3 по модулю 4.
func primeIs3Mod4(p *big.Int) bool {
	return new(big.Int).And(p, big.NewInt(3)).Cmp(big.NewInt(3)) == 0
}

// curveRightHandSide вычисляет f(x) = x^3 + a*x + b (mod p).
func curveRightHandSide(c *ec.Curve, x *big.Int) *big.Int {
	fx := new(big.Int).Mul(x, x)
	fx.Mul(fx, x)
	fx.Add(fx, new(big.Int).Mul(c.A, x))
	fx.Add(fx, c.B)
	fx.Mod(fx, c.P)
	return fx
}

// squareRoot возвращает квадратный корень из f по модулю p = 3 (mod 4) по
// формуле f^((p+1)/4). ok = false, если f не является квадратичным вычетом.
// Отдельно обрабатывается f = 0: корень равен нулю, это точка порядка 2.
func squareRoot(p, f *big.Int) (*big.Int, bool) {
	if f.Sign() == 0 {
		return big.NewInt(0), true
	}

	// Критерий Эйлера: f^((p-1)/2) = 1 для ненулевого квадратичного
	// вычета и -1 иначе.
	legendre := new(big.Int).Exp(f, new(big.Int).Rsh(new(big.Int).Sub(p, one), 1), p)
	if legendre.Cmp(one) != 0 {
		return nil, false
	}

	// f^((p+1)/4): при p = 3 (mod 4) показатель целый, а квадрат корня
	// равен f.
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

// randomScalar выбирает k из диапазона 1..n-1, читая байты из rnd.
func randomScalar(n *big.Int, rnd io.Reader) (*big.Int, error) {
	for {
		k, err := rand.Int(rnd, new(big.Int).Sub(n, one))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNonce, err)
		}
		if k.Sign() != 0 {
			return k, nil
		}
	}
}

// coordinateWidth возвращает ширину координаты в байтах, |p|/8.
func coordinateWidth(c *ec.Curve) int {
	return (c.P.BitLen() + 7) / 8
}

// appendPoint дописывает к buf координаты точки x||y фиксированной ширины.
func appendPoint(buf []byte, p ec.Point, width int) []byte {
	start := len(buf)
	buf = append(buf, make([]byte, 2*width)...)
	p.X.FillBytes(buf[start : start+width])
	p.Y.FillBytes(buf[start+width : start+2*width])
	return buf
}

// parseBlock разбирает блок C1||C2 на две точки.
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
