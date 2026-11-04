package ec

import (
	"errors"
	"math/big"
	"testing"
)

func TestParamSetsValidate(t *testing.T) {
	for _, c := range []*Curve{Params256(), Params512()} {
		t.Run(c.Name, func(t *testing.T) {
			if err := c.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if !c.IsOnCurve(c.Base()) {
				t.Fatal("базовая точка не лежит на кривой")
			}
			if !c.ScalarMult(c.N, c.Base()).IsInfinity() {
				t.Fatal("N*G не является бесконечно удалённой точкой")
			}
		})
	}
}

func TestTamperedParametersRejected(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(c *Curve)
	}{
		{"изменён коэффициент a", func(c *Curve) { c.A.Add(c.A, big.NewInt(1)) }},
		{"изменён коэффициент b", func(c *Curve) { c.B.Add(c.B, big.NewInt(1)) }},
		{"изменён порядок n", func(c *Curve) { c.N.Add(c.N, big.NewInt(2)) }},
		{"сдвинута базовая точка", func(c *Curve) { c.GX.Add(c.GX, big.NewInt(1)) }},
		{"нет базовой точки", func(c *Curve) { c.GY = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Params512()
			tt.tamper(c)
			if err := c.Validate(); err == nil {
				t.Fatal("повреждённые параметры прошли проверку")
			}
		})
	}
}

func TestPointArithmeticIdentities(t *testing.T) {
	c := Params256()
	g := c.Base()

	tests := []struct {
		name string
		got  Point
		want Point
	}{
		{"G + infinity", c.Add(g, Point{}), g},
		{"infinity + G", c.Add(Point{}, g), g},
		{"infinity + infinity", c.Add(Point{}, Point{}), Point{}},
		{"G + G = 2G", c.Add(g, g), c.Double(g)},
		{"G + (-G)", c.Add(g, c.Neg(g)), Point{}},
		{"2G + 3G = 5G", c.Add(c.ScalarMult(big.NewInt(2), g), c.ScalarMult(big.NewInt(3), g)), c.ScalarMult(big.NewInt(5), g)},
		{"3G + 3G = 6G", c.Add(c.ScalarMult(big.NewInt(3), g), c.ScalarMult(big.NewInt(3), g)), c.ScalarMult(big.NewInt(6), g)},
		{"-(2G) = (N-2)G", c.Neg(c.ScalarMult(big.NewInt(2), g)), c.ScalarMult(new(big.Int).Sub(c.N, big.NewInt(2)), g)},
		{"0*G = infinity", c.ScalarMult(big.NewInt(0), g), Point{}},
		{"(-1)*G = -G", c.ScalarMult(big.NewInt(-1), g), c.Neg(g)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.got.Equal(tt.want) {
				t.Fatalf("получено %v, ожидалось %v", tt.got, tt.want)
			}
		})
	}
}

func TestScalarMultOrder(t *testing.T) {
	for _, c := range []*Curve{Params256(), Params512()} {
		t.Run(c.Name, func(t *testing.T) {
			g := c.Base()
			if !c.ScalarMult(new(big.Int).Sub(c.N, big.NewInt(1)), g).Equal(c.Neg(g)) {
				t.Fatal("(N-1)*G должна совпадать с -G")
			}
			if !c.ScalarMult(new(big.Int).Add(c.N, big.NewInt(1)), g).Equal(g) {
				t.Fatal("(N+1)*G должна совпадать с G")
			}
		})
	}
}

func TestIsOnCurveRejectsForeignPoints(t *testing.T) {
	c := Params256()
	if c.IsOnCurve(Point{X: big.NewInt(1), Y: big.NewInt(1)}) {
		t.Fatal("посторонняя точка признана лежащей на кривой")
	}
	if c.IsOnCurve(Point{X: new(big.Int).Add(c.P, big.NewInt(1)), Y: big.NewInt(0)}) {
		t.Fatal("координата вне поля признана допустимой")
	}
	if c.IsOnCurve(Point{X: big.NewInt(1)}) {
		t.Fatal("точка с одной координатой признана допустимой")
	}
}

func TestValidateRejectsNilCurve(t *testing.T) {
	var c *Curve
	if !errors.Is(c.Validate(), ErrInvalidParams) {
		t.Fatal("ожидалась ошибка ErrInvalidParams")
	}
}
