package mceliece

// Многочлены над GF(2^m).
//
// Poly — коэффициенты по возрастанию степени: coeff[i] — коэффициент при z^i.
// Хвостовые нули обрезаны, поэтому len(p) == deg(p)+1, а нулевой многочлен —
// пустой срез (deg = -1). Такое представление делает сравнение и деление
// многочленов простыми: нулевые старшие коэффициенты просто не хранятся.

// Poly — многочлен над GF(2^m).
type Poly []uint16

// Deg возвращает степень многочлена или -1 для нулевого.
func Deg(p Poly) int { return len(p) - 1 }

// PolyNormalize обрезает хвостовые нули.
func (f *Field) PolyNormalize(p Poly) Poly {
	n := len(p)
	for n > 0 && p[n-1] == 0 {
		n--
	}
	return p[:n]
}

// PolyCopy возвращает независимую копию многочлена.
func PolyCopy(p Poly) Poly {
	if len(p) == 0 {
		return nil
	}
	return append(Poly(nil), p...)
}

// PolyAdd складывает многочлены; в характеристике 2 это XOR коэффициентов.
func (f *Field) PolyAdd(a, b Poly) Poly {
	n := max(len(a), len(b))
	sum := make(Poly, n)
	for i := 0; i < n; i++ {
		var x, y uint16
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		sum[i] = f.Add(x, y)
	}
	return f.PolyNormalize(sum)
}

// PolyMul перемножает многочлены.
func (f *Field) PolyMul(a, b Poly) Poly {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	product := make(Poly, len(a)+len(b)-1)
	for i, x := range a {
		if x == 0 {
			continue
		}
		for j, y := range b {
			product[i+j] ^= f.Mul(x, y)
		}
	}
	return f.PolyNormalize(product)
}

// PolyDivMod делит a на b с остатком: a = q*b + r, deg r < deg b.
func (f *Field) PolyDivMod(a, b Poly) (Poly, Poly) {
	if len(b) == 0 {
		panic("mceliece: polynomial division by zero")
	}
	if len(a) < len(b) {
		return nil, PolyCopy(a)
	}

	q := make(Poly, len(a)-len(b)+1)
	r := PolyCopy(a)
	invLead := f.Inv(b[len(b)-1])

	for len(r) >= len(b) {
		shift := len(r) - len(b)
		// Старший коэффициент остатка уничтожается точно, коэффициент
		// частного — это r/(старший b).
		coef := f.Mul(r[len(r)-1], invLead)
		q[shift] = coef
		for i := range b {
			r[shift+i] ^= f.Mul(coef, b[i])
		}
		r = f.PolyNormalize(r)
	}
	return f.PolyNormalize(q), r
}

// PolyMod возвращает остаток от деления a на b.
func (f *Field) PolyMod(a, b Poly) Poly {
	_, r := f.PolyDivMod(a, b)
	return r
}

// PolyGCD возвращает нормированный наибольший общий делитель.
func (f *Field) PolyGCD(a, b Poly) Poly {
	a, b = PolyCopy(a), PolyCopy(b)
	for len(b) != 0 {
		a, b = b, f.PolyMod(a, b)
	}
	return f.makeMonic(a)
}

// makeMonic делит многочлен на старший коэффициент.
func (f *Field) makeMonic(p Poly) Poly {
	if len(p) == 0 {
		return nil
	}
	inv := f.Inv(p[len(p)-1])
	if inv == 1 {
		return p
	}
	monic := make(Poly, len(p))
	for i := range p {
		monic[i] = f.Mul(p[i], inv)
	}
	return monic
}

// PolyExtGCD возвращает g, u, v, для которых u*a + v*b = g.
func (f *Field) PolyExtGCD(a, b Poly) (Poly, Poly, Poly) {
	// Инвариант: r = u*a + v*b.
	r0, r1 := PolyCopy(a), PolyCopy(b)
	u0, u1 := Poly{1}, Poly(nil)
	v0, v1 := Poly(nil), Poly{1}

	for len(r1) != 0 {
		q, r2 := f.PolyDivMod(r0, r1)
		u2 := f.PolyAdd(u0, f.PolyMul(q, u1))
		v2 := f.PolyAdd(v0, f.PolyMul(q, v1))
		r0, r1 = r1, r2
		u0, u1 = u1, u2
		v0, v1 = v1, v2
	}

	g := f.makeMonic(r0)
	if len(r0) == 0 {
		return nil, nil, nil
	}
	invLead := f.Inv(r0[len(r0)-1])
	for i := range u0 {
		u0[i] = f.Mul(u0[i], invLead)
	}
	for i := range v0 {
		v0[i] = f.Mul(v0[i], invLead)
	}
	return g, f.PolyNormalize(u0), f.PolyNormalize(v0)
}

// PolyInv возвращает a^{-1} mod modulus; ошибка означает, что многочлены
// не взаимно просты.
func (f *Field) PolyInv(a, modulus Poly) (Poly, error) {
	g, u, _ := f.PolyExtGCD(a, modulus)
	if Deg(g) != 0 {
		return nil, ErrNotInvertible
	}
	// u*a ≡ g (mod modulus), а g — ненулевая константа после нормировки.
	return f.PolyMod(u, modulus), nil
}

// PolyEval вычисляет многочлен в точке x по схеме Горнера.
func (f *Field) PolyEval(p Poly, x uint16) uint16 {
	var value uint16
	for i := len(p) - 1; i >= 0; i-- {
		value = f.Add(f.Mul(value, x), p[i])
	}
	return value
}

// PolySquare возводит многочлен в квадрат: (Σ a_i z^i)^2 = Σ a_i^2 z^(2i),
// потому что в характеристике 2 все промежуточные члены сокращаются.
func (f *Field) PolySquare(p Poly) Poly {
	if len(p) == 0 {
		return nil
	}
	squared := make(Poly, 2*len(p)-1)
	for i, a := range p {
		squared[2*i] = f.Square(a)
	}
	return f.PolyNormalize(squared)
}

// PolySqrtMod извлекает квадратный корень из c по модулю g: возвращает T
// такой, что T^2 ≡ c (mod g).
//
// g неприводим степени t, поэтому GF(2^m)[z]/g — поле GF(2^(m*t)), где
// x^(2^(m*t)) = x. Обратная к возведению в квадрат операция — это
// x^(2^(m*t-1)), то есть m*t-1 последовательных возведений в квадрат.
func (f *Field) PolySqrtMod(c, g Poly) (Poly, error) {
	if len(g) == 0 || g[len(g)-1] == 0 {
		return nil, ErrNotInvertible
	}
	root := f.PolyMod(c, g)
	for i := 0; i < f.m*Deg(g)-1; i++ {
		root = f.PolyMod(f.PolySquare(root), g)
	}
	return root, nil
}

// frobenius возводит p в степень 2^m по модулю g: один шаг x → x^(2^m).
func (f *Field) frobenius(p, g Poly) Poly {
	for i := 0; i < f.m; i++ {
		p = f.PolyMod(f.PolySquare(p), g)
	}
	return p
}
