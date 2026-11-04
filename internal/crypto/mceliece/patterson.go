package mceliece

// Декодирование бинарного кода Гоппы алгоритмом Паттерсона.
//
// Вход — принятое слово r = c + e, где c — кодовое слово, а e содержит не
// более t ошибок. Синдром S(z) = Σ e_i/(z - a_i) mod g равен нулю тогда и
// только тогда, когда ошибок нет. Далее по S строится многочлен
// локаторов ошибок σ(z) = a(z)^2 + z*b(z)^2, корни которого — опорные
// элементы с ошибками.
//
// Примечание о квадратном корне: z + S^{-1} не раскладывается на чётные
// степени z напрямую, поэтому корень извлекается в фактор-кольце
// GF(2^m)[z]/g: для неприводимого g это поле GF(2^(m*t)), в котором
// возведение в квадрат — биекция с обратной x^(2^(m*t-1)).

// Decoder — декодер Паттерсона для кода Гоппы, заданного многочленом g и
// упорядоченным списком опорных элементов.
type Decoder struct {
	Field   *Field
	Params  Params
	G       Poly
	Support []uint16
}

// Decode исправляет до t ошибок в принятом слове и возвращает кодовое слово.
// Если ошибок больше t или синдром не согласован, возвращается ErrDecode.
func (d *Decoder) Decode(received []uint64) ([]uint64, error) {
	if len(d.Support) != d.Params.N || len(received)*64 < d.Params.N {
		return nil, ErrDecode
	}

	syndrome, err := d.syndrome(received)
	if err != nil {
		return nil, err
	}
	if len(syndrome) == 0 {
		return append([]uint64(nil), received...), nil
	}

	// T(z) = sqrt(z + S(z)^{-1}) mod g.
	inverse, err := d.Field.PolyInv(syndrome, d.G)
	if err != nil {
		return nil, err
	}
	target := d.Field.PolyAdd(Poly{0, 1}, inverse)
	t, err := d.Field.PolySqrtMod(target, d.G)
	if err != nil {
		return nil, err
	}

	// Расширенный алгоритм Евклида для (g, T); останавливаемся на первом
	// остатке степени не выше t/2.
	a, b := d.euclidean(t, Deg(d.G)/2)

	// σ(z) = a(z)^2 + z*b(z)^2.
	sigma := d.Field.PolyAdd(d.Field.PolySquare(a), d.Field.PolyMul(Poly{0, 1}, d.Field.PolySquare(b)))

	errors := newVector(d.Params.N)
	found := 0
	for position, element := range d.Support {
		if d.Field.PolyEval(sigma, element) == 0 {
			setVectorBit(errors, position)
			found++
		}
	}
	// У корректного синдрома σ раскладывается на столько линейных
	// множителей, какова его степень; иначе исправление невозможно.
	if found == 0 || found != Deg(sigma) {
		return nil, ErrDecode
	}

	corrected := make([]uint64, len(received))
	for i := range received {
		corrected[i] = received[i] ^ errors[i]
	}
	return corrected, nil
}

// syndrome вычисляет S(z) = Σ_{r_i=1} 1/(z - a_i) mod g.
func (d *Decoder) syndrome(received []uint64) (Poly, error) {
	sum := Poly(nil)
	for position, element := range d.Support {
		if !vectorBit(received, position) {
			continue
		}
		// 1/(z - a) = 1/(z + a) в характеристике 2.
		inverse, err := d.Field.PolyInv(Poly{element, 1}, d.G)
		if err != nil {
			return nil, err
		}
		sum = d.Field.PolyAdd(sum, inverse)
	}
	return sum, nil
}

// euclidean выполняет расширенный алгоритм Евклида для g и t и возвращает
// пару (a, b), для которой a ≡ b*T (mod g) и deg a, deg b <= t/2.
func (d *Decoder) euclidean(t Poly, half int) (Poly, Poly) {
	rPrev, rCur := d.G, t
	bPrev, bCur := Poly(nil), Poly{1}

	for Deg(rCur) > half {
		q, rNext := d.Field.PolyDivMod(rPrev, rCur)
		bNext := d.Field.PolyAdd(bPrev, d.Field.PolyMul(q, bCur))
		rPrev, rCur = rCur, rNext
		bPrev, bCur = bCur, bNext
	}
	return rCur, bCur
}
