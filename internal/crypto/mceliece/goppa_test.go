package mceliece

import "testing"

func TestGoppaCodeProperties(t *testing.T) {
	params := mustParams(t, 8, 8)
	code, err := NewGoppaCode(params, deterministicRandom(1))
	if err != nil {
		t.Fatalf("NewGoppaCode: %v", err)
	}

	if code.Field.Degree() != params.M || Deg(code.G) != params.T {
		t.Fatalf("unexpected parameters: field %d, deg g %d", code.Field.Degree(), Deg(code.G))
	}
	if !isIrreducible(code.Field, code.G, params.T) {
		t.Fatal("Goppa polynomial is not irreducible")
	}

	if rank := code.Matrix.Rank(); rank != params.K {
		t.Fatalf("rank G = %d, want %d", rank, params.K)
	}
	for row := 0; row < params.K; row++ {
		for column := 0; column < params.K; column++ {
			want := row == column
			if code.Matrix.At(row, column) != want {
				t.Fatalf("G[%d][%d] = %v, want %v (not systematic)", row, column, !want, want)
			}
		}
	}

	// Проверочная матрица, восстановленная по g и support, должна
	// обнулять порождающую: H*G^T = 0.
	h, err := parityCheckMatrix(code.Field, code.G, code.Support)
	if err != nil {
		t.Fatalf("parity check matrix: %v", err)
	}
	for row := 0; row < h.Rows; row++ {
		for generator := 0; generator < params.K; generator++ {
			var dot uint16
			for column := 0; column < params.N; column++ {
				if h.At(row, column) && code.Matrix.At(generator, column) {
					dot ^= 1
				}
			}
			if dot != 0 {
				t.Fatalf("H*G^T is not zero at (%d, %d)", row, generator)
			}
		}
	}

	seen := make([]bool, params.N)
	for _, element := range code.Support {
		if int(element) >= params.N || seen[element] {
			t.Fatalf("support is not a permutation: element %d", element)
		}
		seen[element] = true
	}
}

func TestGoppaCodeRejectsBadSupport(t *testing.T) {
	field := mustField(t, 8)
	g, err := RandomIrreduciblePolynomial(field, 8, deterministicRandom(3))
	if err != nil {
		t.Fatalf("irreducible polynomial: %v", err)
	}
	if _, err := BuildGoppaCode(field, g, field.Elements()[:100]); err == nil {
		t.Fatal("BuildGoppaCode accepted a short support")
	}
}

func TestKeyGenerationScramblesWithS(t *testing.T) {
	params := mustParams(t, 8, 8)
	public, private, err := GenerateKeyPair(params, deterministicRandom(2))
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// G1 = S*G, где S = (S^{-1})^{-1}.
	field := mustField(t, params.M)
	code, err := BuildGoppaCode(field, private.G, private.Support)
	if err != nil {
		t.Fatalf("rebuild code: %v", err)
	}
	scrambler, ok := private.SInverse.Invert()
	if !ok {
		t.Fatal("S^{-1} is singular")
	}
	expected, err := scrambler.Mul(code.Matrix)
	if err != nil {
		t.Fatalf("S*G: %v", err)
	}
	if !public.Matrix.Equal(expected) {
		t.Fatal("public matrix is not S*G")
	}
}

func TestNewParams(t *testing.T) {
	cases := []struct {
		m, degree int
		ok        bool
	}{
		{8, 8, true},
		{10, 38, true},
		{7, 8, false},
		{8, 0, false},
		{8, 32, false},
	}
	for _, c := range cases {
		params, err := NewParams(c.m, c.degree)
		if c.ok && err != nil {
			t.Errorf("NewParams(%d, %d): %v", c.m, c.degree, err)
		}
		if !c.ok {
			if err == nil {
				t.Errorf("NewParams(%d, %d) succeeded, want an error", c.m, c.degree)
			}
			continue
		}
		if params.N != 1<<c.m || params.K != params.N-c.m*c.degree {
			t.Errorf("NewParams(%d, %d) = %+v", c.m, c.degree, params)
		}
	}
}

func TestMatrixInvertAndRank(t *testing.T) {
	matrix, err := randomMatrix(6, 6, deterministicRandom(5))
	if err != nil {
		t.Fatalf("random matrix: %v", err)
	}
	if matrix.Rank() == 6 {
		inverse, ok := matrix.Invert()
		if !ok {
			t.Fatal("invertible matrix rejected")
		}
		product, err := matrix.Mul(inverse)
		if err != nil {
			t.Fatalf("multiply: %v", err)
		}
		for i := 0; i < 6; i++ {
			for j := 0; j < 6; j++ {
				if product.At(i, j) != (i == j) {
					t.Fatalf("A*A^{-1} is not identity at (%d, %d)", i, j)
				}
			}
		}
		return
	}
	if _, ok := matrix.Invert(); ok {
		t.Fatal("singular matrix reported as invertible")
	}
}

func TestMatrixSerialization(t *testing.T) {
	matrix := NewMatrix(5, 11)
	matrix.Set(0, 0, true)
	matrix.Set(1, 10, true)
	matrix.Set(4, 5, true)

	decoded, err := UnmarshalMatrix(5, 11, matrix.MarshalBinary())
	if err != nil {
		t.Fatalf("UnmarshalMatrix: %v", err)
	}
	if !decoded.Equal(matrix) {
		t.Fatal("matrix changed after serialization round trip")
	}
	if _, err := UnmarshalMatrix(5, 11, matrix.MarshalBinary()[:5]); err == nil {
		t.Fatal("truncated matrix accepted")
	}
}
