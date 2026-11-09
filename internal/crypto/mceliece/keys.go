package mceliece

import "encoding/binary"

// Двоичный формат ключей.
//
// Открытый ключ:
//
//	[0:8]   сигнатура "MZIMCEPK"
//	[8]     версия формата
//	[9]     m — поле GF(2^m)
//	[10:12] t — степень многочлена Гоппы
//	[12:14] n — длина кода
//	[14:16] k — размерность кода
//	[16:20] длина матрицы G1 в байтах
//	[20:]   G1: k строк по (n+7)/8 байт, старший бит вперёд
//
// Закрытый ключ:
//
//	[0:8]   сигнатура "MZIMCESK"
//	[8]     версия формата
//	[9]     m
//	[10:12] t
//	[12:14] n
//	[14:16] k
//	[16:18] число коэффициентов многочлена g
//	[18:20] число опорных элементов
//	[20:24] длина матрицы S^{-1} в байтах
//	[24:]   g, support и S^{-1} подряд: коэффициенты uint16 и биты
//	        матрицы k×k, старший бит вперёд, по (k+7)/8 байт на строку.
//
// Все числа записаны в старшем порядке байтов. Длина файла сверяется с
// параметрами: ни один раздел не может быть обрезан или расширен.

func (k *PublicKey) MarshalBinary() []byte {
	matrix := k.Matrix.MarshalBinary()
	out := make([]byte, publicHeaderSize+len(matrix))
	copy(out, publicMagic)
	out[8] = keyVersion
	out[9] = byte(k.Params.M)
	binary.BigEndian.PutUint16(out[10:], uint16(k.Params.T))
	binary.BigEndian.PutUint16(out[12:], uint16(k.Params.N))
	binary.BigEndian.PutUint16(out[14:], uint16(k.Params.K))
	binary.BigEndian.PutUint32(out[16:], uint32(len(matrix)))
	copy(out[publicHeaderSize:], matrix)
	return out
}

// ParsePublicKey разбирает открытый ключ и проверяет все размеры.
func ParsePublicKey(data []byte) (*PublicKey, error) {
	params, err := parseKeyHeader(data, publicMagic)
	if err != nil {
		return nil, err
	}
	if len(data) < publicHeaderSize {
		return nil, ErrBadPublicKey
	}
	payloadLen := binary.BigEndian.Uint32(data[16:publicHeaderSize])
	rowBytes := (params.N + 7) / 8
	if int(payloadLen) != params.K*rowBytes || len(data) != publicHeaderSize+int(payloadLen) {
		return nil, ErrBadPublicKey
	}

	matrix, err := UnmarshalMatrix(params.K, params.N, data[publicHeaderSize:])
	if err != nil {
		return nil, ErrBadPublicKey
	}
	return &PublicKey{Params: params, Matrix: matrix}, nil
}

func (k *PrivateKey) MarshalBinary() []byte {
	matrix := k.SInverse.MarshalBinary()
	size := privateHeaderSize + 2*len(k.G) + 2*len(k.Support) + len(matrix)
	out := make([]byte, size)
	copy(out, privateMagic)
	out[8] = keyVersion
	out[9] = byte(k.Params.M)
	binary.BigEndian.PutUint16(out[10:], uint16(k.Params.T))
	binary.BigEndian.PutUint16(out[12:], uint16(k.Params.N))
	binary.BigEndian.PutUint16(out[14:], uint16(k.Params.K))
	binary.BigEndian.PutUint16(out[16:], uint16(len(k.G)))
	binary.BigEndian.PutUint16(out[18:], uint16(len(k.Support)))
	binary.BigEndian.PutUint32(out[20:], uint32(len(matrix)))

	offset := privateHeaderSize
	for _, coefficient := range k.G {
		binary.BigEndian.PutUint16(out[offset:], coefficient)
		offset += 2
	}
	for _, element := range k.Support {
		binary.BigEndian.PutUint16(out[offset:], element)
		offset += 2
	}
	copy(out[offset:], matrix)
	return out
}

// ParsePrivateKey разбирает закрытый ключ и проверяет размеры, степень g,
// диапазон опорных элементов и то, что support — перестановка всех
// элементов поля.
func ParsePrivateKey(data []byte) (*PrivateKey, error) {
	params, err := parseKeyHeader(data, privateMagic)
	if err != nil {
		return nil, err
	}
	if len(data) < privateHeaderSize {
		return nil, ErrBadPrivateKey
	}

	gLength := int(binary.BigEndian.Uint16(data[16:]))
	supportLength := int(binary.BigEndian.Uint16(data[18:]))
	matrixLength := int(binary.BigEndian.Uint32(data[20:privateHeaderSize]))
	expected := privateHeaderSize + 2*gLength + 2*supportLength + matrixLength
	if gLength != params.T+1 || supportLength != params.N || len(data) != expected {
		return nil, ErrBadPrivateKey
	}

	offset := privateHeaderSize
	g := make(Poly, gLength)
	for i := range g {
		g[i] = binary.BigEndian.Uint16(data[offset:])
		offset += 2
	}
	if g[len(g)-1] != 1 {
		return nil, ErrBadPrivateKey
	}
	for _, coefficient := range g {
		if int(coefficient) >= 1<<params.M {
			return nil, ErrBadPrivateKey
		}
	}

	support := make([]uint16, supportLength)
	seen := make([]bool, params.N)
	for i := range support {
		element := binary.BigEndian.Uint16(data[offset:])
		offset += 2
		if int(element) >= params.N || seen[element] {
			return nil, ErrBadPrivateKey
		}
		seen[element] = true
		support[i] = element
	}

	matrix, err := UnmarshalMatrix(params.K, params.K, data[offset:])
	if err != nil {
		return nil, ErrBadPrivateKey
	}
	return &PrivateKey{Params: params, G: g, Support: support, SInverse: matrix}, nil
}

func parseKeyHeader(data []byte, magic string) (Params, error) {
	if len(data) < publicHeaderSize || string(data[:len(magic)]) != magic || data[8] != keyVersion {
		return Params{}, ErrBadPublicKey
	}
	params, err := NewParams(int(data[9]), int(binary.BigEndian.Uint16(data[10:])))
	if err != nil {
		return Params{}, ErrBadPublicKey
	}
	n := int(binary.BigEndian.Uint16(data[12:]))
	k := int(binary.BigEndian.Uint16(data[14:]))
	if n != params.N || k != params.K {
		return Params{}, ErrBadPublicKey
	}
	return params, nil
}
