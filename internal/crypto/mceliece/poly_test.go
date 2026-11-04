package mceliece

import "testing"

func TestPolyDivMod(t *testing.T) {
	field := mustField(t, 8)
	cases := []struct {
		a, b Poly
	}{
		{Poly{3, 0, 5, 1, 200}, Poly{7, 2, 1}},
		{Poly{1, 1}, Poly{1, 1}},
		{Poly{9}, Poly{4, 1}},
		{Poly{1, 2, 3, 4, 5, 6}, Poly{1}},
	}
	for _, c := range cases {
		q, r := field.PolyDivMod(c.a, c.b)
		rebuilt := field.PolyAdd(field.PolyMul(q, c.b), r)
		if !field.PolyEqual(rebuilt, c.a) {
			t.Errorf("a = q*b + r fails: %v != %v", rebuilt, c.a)
		}
		if Deg(r) >= Deg(c.b) {
			t.Errorf("deg r = %d, want < %d", Deg(r), Deg(c.b))
		}
	}
}

func TestPolyGCDAndExtGCD(t *testing.T) {
	field := mustField(t, 8)
	a := Poly{3, 0, 5, 1, 200}
	b := Poly{7, 2, 1}

	g, u, v := field.PolyExtGCD(a, b)
	combination := field.PolyAdd(field.PolyMul(u, a), field.PolyMul(v, b))
	if !field.PolyEqual(combination, g) {
		t.Fatalf("u*a + v*b = %v, want %v", combination, g)
	}
	if !field.PolyEqual(g, field.PolyGCD(a, b)) {
		t.Fatalf("extgcd %v differs from gcd %v", g, field.PolyGCD(a, b))
	}
	if g[len(g)-1] != 1 {
		t.Fatalf("gcd is not monic: %v", g)
	}
}

func TestPolySquareAndSqrtMod(t *testing.T) {
	field := mustField(t, 8)
	g, err := RandomIrreduciblePolynomial(field, 4, deterministicRandom(7))
	if err != nil {
		t.Fatalf("irreducible polynomial: %v", err)
	}

	for _, c := range []Poly{{5, 1, 9}, {0}, {255, 255, 1, 2}} {
		squared := field.PolyMod(field.PolySquare(c), g)
		root, err := field.PolySqrtMod(squared, g)
		if err != nil {
			t.Fatalf("sqrt: %v", err)
		}
		if !field.PolyEqual(root, field.PolyMod(c, g)) {
			t.Errorf("sqrt(square(%v)) = %v, want %v", c, root, field.PolyMod(c, g))
		}
	}
}

func TestPolyInv(t *testing.T) {
	field := mustField(t, 8)
	g, err := RandomIrreduciblePolynomial(field, 4, deterministicRandom(11))
	if err != nil {
		t.Fatalf("irreducible polynomial: %v", err)
	}
	a := Poly{3, 0, 1}
	inverse, err := field.PolyInv(a, g)
	if err != nil {
		t.Fatalf("inv: %v", err)
	}
	product := field.PolyMod(field.PolyMul(a, inverse), g)
	if !field.PolyEqual(product, Poly{1}) {
		t.Fatalf("a * a^-1 = %v, want 1", product)
	}
}
