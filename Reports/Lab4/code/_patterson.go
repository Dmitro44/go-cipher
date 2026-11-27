package mceliece

type Decoder struct {
	Field   *Field
	Params  Params
	G       Poly
	Support []uint16
}

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

	inverse, err := d.Field.PolyInv(syndrome, d.G)
	if err != nil {
		return nil, err
	}
	target := d.Field.PolyAdd(Poly{0, 1}, inverse)
	t, err := d.Field.PolySqrtMod(target, d.G)
	if err != nil {
		return nil, err
	}

	a, b := d.euclidean(t, Deg(d.G)/2)

	sigma := d.Field.PolyAdd(d.Field.PolySquare(a), d.Field.PolyMul(Poly{0, 1}, d.Field.PolySquare(b)))

	errors := newVector(d.Params.N)
	found := 0
	for position, element := range d.Support {
		if d.Field.PolyEval(sigma, element) == 0 {
			setVectorBit(errors, position)
			found++
		}
	}
	if found == 0 || found != Deg(sigma) {
		return nil, ErrDecode
	}

	corrected := make([]uint64, len(received))
	for i := range received {
		corrected[i] = received[i] ^ errors[i]
	}
	return corrected, nil
}

func (d *Decoder) syndrome(received []uint64) (Poly, error) {
	sum := Poly(nil)
	for position, element := range d.Support {
		if !vectorBit(received, position) {
			continue
		}
		inverse, err := d.Field.PolyInv(Poly{element, 1}, d.G)
		if err != nil {
			return nil, err
		}
		sum = d.Field.PolyAdd(sum, inverse)
	}
	return sum, nil
}

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
