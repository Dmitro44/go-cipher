// Пакет ec реализует арифметику точек эллиптической кривой над простым
// полем в короткой форме Вейерштрасса
//
//	y^2 = x^3 + a*x + b (mod p).
//
// Точки хранятся в аффинных координатах. Бесконечно удалённая точка
// изображается значением Point{} — обе координаты nil. Пакет не содержит
// ничего, что относилось бы к конкретной схеме подписи: им пользуются и
// ГОСТ Р 34.10-2012, и схема Эль-Гамаля на эллиптических кривых.
package ec

import (
	"errors"
	"fmt"
	"math/big"
)

var (
	// ErrInvalidParams — набор параметров кривой не прошёл проверку.
	ErrInvalidParams = errors.New("ec: invalid curve parameters")
	// ErrPointNotOnCurve — точка не лежит на кривой.
	ErrPointNotOnCurve = errors.New("ec: point is not on the curve")
)

// Point — точка кривой в аффинных координатах. Если X и Y равны nil,
// точка бесконечно удалённая.
type Point struct {
	X, Y *big.Int
}

// IsInfinity сообщает, является ли точка бесконечно удалённой.
func (p Point) IsInfinity() bool {
	return p.X == nil && p.Y == nil
}

// Equal сравнивает две точки.
func (p Point) Equal(q Point) bool {
	if p.IsInfinity() || q.IsInfinity() {
		return p.IsInfinity() && q.IsInfinity()
	}
	return p.X.Cmp(q.X) == 0 && p.Y.Cmp(q.Y) == 0
}

// Curve — параметры эллиптической кривой и порядок N базовой точки.
//
//	GX, GY — координаты базовой точки G;
//	P      — модуль поля (простое число);
//	A, B   — коэффициенты уравнения кривой;
//	N      — порядок точки G (простое число).
type Curve struct {
	Name string
	P    *big.Int
	A    *big.Int
	B    *big.Int
	N    *big.Int
	GX   *big.Int
	GY   *big.Int
}

// Base возвращает базовую точку кривой.
func (c *Curve) Base() Point {
	return Point{X: c.GX, Y: c.GY}
}

// Add складывает точки P и Q по правилу секущей:
//
//	lambda = (y2 - y1) / (x2 - x1) (mod p),
//	x3 = lambda^2 - x1 - x2,
//	y3 = lambda*(x1 - x3) - y1.
//
// Особые случаи: бесконечно удалённая точка — нейтральный элемент,
// а сумма точки и ей противоположной — бесконечно удалённая точка.
func (c *Curve) Add(p, q Point) Point {
	if p.IsInfinity() {
		return q
	}
	if q.IsInfinity() {
		return p
	}

	if p.X.Cmp(q.X) == 0 {
		// При одинаковых x точки либо совпадают (удвоение), либо
		// являются взаимно противоположными.
		if p.Y.Cmp(q.Y) != 0 {
			return Point{}
		}
		return c.Double(p)
	}

	// lambda = (y2 - y1) / (x2 - x1) mod p.
	num := new(big.Int).Sub(q.Y, p.Y)
	den := new(big.Int).Sub(q.X, p.X)
	den.ModInverse(den, c.P)
	if den == nil {
		// Обратного элемента нет только при составном p; такая кривая
		// отвергается в Validate.
		return Point{}
	}
	lambda := new(big.Int).Mul(num, den)
	lambda.Mod(lambda, c.P)

	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, p.X)
	x3.Sub(x3, q.X)
	x3.Mod(x3, c.P)

	y3 := new(big.Int).Sub(p.X, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, p.Y)
	y3.Mod(y3, c.P)

	return Point{X: x3, Y: y3}
}

// Double удваивает точку по правилу касательной:
//
//	lambda = (3*x^2 + a) / (2*y) (mod p),
//	x3 = lambda^2 - 2*x,
//	y3 = lambda*(x - x3) - y.
//
// Точка с y = 0 имеет порядок 2, поэтому её удвоение — бесконечно
// удалённая точка.
func (c *Curve) Double(p Point) Point {
	if p.IsInfinity() || p.Y.Sign() == 0 {
		return Point{}
	}

	// lambda = (3*x^2 + a) / (2*y) mod p.
	num := new(big.Int).Mul(p.X, p.X)
	num.Mul(num, big.NewInt(3))
	num.Add(num, c.A)
	num.Mod(num, c.P)

	den := new(big.Int).Lsh(p.Y, 1)
	den.ModInverse(den, c.P)
	if den == nil {
		return Point{}
	}
	lambda := new(big.Int).Mul(num, den)
	lambda.Mod(lambda, c.P)

	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, p.X)
	x3.Sub(x3, p.X)
	x3.Mod(x3, c.P)

	y3 := new(big.Int).Sub(p.X, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, p.Y)
	y3.Mod(y3, c.P)

	return Point{X: x3, Y: y3}
}

// Neg возвращает точку, противоположную данной: (x, -y mod p).
func (c *Curve) Neg(p Point) Point {
	if p.IsInfinity() {
		return Point{}
	}
	y := new(big.Int).Neg(p.Y)
	y.Mod(y, c.P)
	return Point{X: new(big.Int).Set(p.X), Y: y}
}

// ScalarMult вычисляет k*P методом «удвоение и сложение»: двоичное
// представление k просматривается от младшего бита к старшему, текущая
// точка при каждом шаге удваивается, а при единичном бите добавляется к
// результату. Для отрицательного k вычисляется |k|*(-P).
func (c *Curve) ScalarMult(k *big.Int, p Point) Point {
	if k == nil || k.Sign() == 0 || p.IsInfinity() {
		return Point{}
	}

	scalar := k
	if scalar.Sign() < 0 {
		scalar = new(big.Int).Neg(scalar)
		p = c.Neg(p)
	}

	result := Point{}
	addend := p
	for i := 0; i < scalar.BitLen(); i++ {
		if scalar.Bit(i) == 1 {
			result = c.Add(result, addend)
		}
		addend = c.Double(addend)
	}
	return result
}

// IsOnCurve проверяет уравнение y^2 = x^3 + a*x + b (mod p) и то, что
// координаты приведены по модулю p. Бесконечно удалённая точка считается
// лежащей на кривой.
func (c *Curve) IsOnCurve(p Point) bool {
	if p.IsInfinity() {
		return true
	}
	if p.X == nil || p.Y == nil {
		return false
	}
	if p.X.Sign() < 0 || p.X.Cmp(c.P) >= 0 || p.Y.Sign() < 0 || p.Y.Cmp(c.P) >= 0 {
		return false
	}

	left := new(big.Int).Mul(p.Y, p.Y)
	left.Mod(left, c.P)

	right := new(big.Int).Mul(p.X, p.X)
	right.Mul(right, p.X)
	right.Add(right, new(big.Int).Mul(c.A, p.X))
	right.Add(right, c.B)
	right.Mod(right, c.P)

	return left.Cmp(right) == 0
}

// Validate проверяет, что параметры кривой корректны: коэффициенты и
// координаты базовой точки лежат в поле, G удовлетворяет уравнению
// кривой и порядок N действительно аннулирует G (N*G — бесконечно
// удалённая точка).
func (c *Curve) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: curve is nil", ErrInvalidParams)
	}
	if c.P == nil || c.A == nil || c.B == nil || c.N == nil || c.GX == nil || c.GY == nil {
		return fmt.Errorf("%w: missing parameters", ErrInvalidParams)
	}
	if c.P.Sign() <= 0 || c.P.Bit(0) == 0 {
		return fmt.Errorf("%w: modulus must be an odd positive number", ErrInvalidParams)
	}
	if c.A.Sign() < 0 || c.A.Cmp(c.P) >= 0 {
		return fmt.Errorf("%w: coefficient a out of range", ErrInvalidParams)
	}
	if c.B.Sign() < 0 || c.B.Cmp(c.P) >= 0 {
		return fmt.Errorf("%w: coefficient b out of range", ErrInvalidParams)
	}
	if c.N.Cmp(big.NewInt(1)) <= 0 {
		return fmt.Errorf("%w: order must be greater than one", ErrInvalidParams)
	}
	if !c.IsOnCurve(c.Base()) {
		return fmt.Errorf("%w: base point", ErrPointNotOnCurve)
	}
	if !c.ScalarMult(c.N, c.Base()).IsInfinity() {
		return fmt.Errorf("%w: N*G is not the point at infinity", ErrInvalidParams)
	}
	return nil
}

// clone возвращает независимую копию параметров, чтобы вызывающий код не
// мог изменить общие big.Int.
func (c *Curve) clone() *Curve {
	return &Curve{
		Name: c.Name,
		P:    new(big.Int).Set(c.P),
		A:    new(big.Int).Set(c.A),
		B:    new(big.Int).Set(c.B),
		N:    new(big.Int).Set(c.N),
		GX:   new(big.Int).Set(c.GX),
		GY:   new(big.Int).Set(c.GY),
	}
}
