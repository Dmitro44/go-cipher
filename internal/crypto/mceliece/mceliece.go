package mceliece

import (
	"crypto/rand"
	"fmt"
	"io"
)

// Криптосистема Мак-Элиса.
//
// Открытый ключ — порождающая матрица G1 = S*G*P, где G — систематическая
// порождающая матрица кода Гоппы, S — случайная обратимая k×k матрица,
// P — перестановка координат. В реализации P объединена со случайным
// порядком опорных элементов, поэтому хранится не матрица перестановки,
// а уже переставленный список опорных элементов support.
//
// Зашифрование: C = M*G1 + Z, где M — k-битный блок сообщения, Z — вектор
// ошибок веса ровно t.
//
// Расшифрование: принятое слово декодируется Паттерсоном с секретными g и
// support, что даёт M*S, затем умножается на S^{-1}.

const (
	publicMagic  = "MZIMCEPK"
	privateMagic = "MZIMCESK"
	keyVersion   = 1

	publicHeaderSize  = 20
	privateHeaderSize = 24
	framingBits       = 32
)

// PublicKey — открытый ключ Мак-Элиса.
type PublicKey struct {
	Params Params
	Matrix *Matrix // G1, k × n
}

// PrivateKey — закрытый ключ Мак-Элиса.
type PrivateKey struct {
	Params   Params
	G        Poly
	Support  []uint16
	SInverse *Matrix // S^{-1}, k × k
}

// GenerateKeyPair создаёт ключевую пару. random — источник случайности;
// в рабочем коде это crypto/rand, в тестах — детерминированный читатель.
func GenerateKeyPair(params Params, random io.Reader) (*PublicKey, *PrivateKey, error) {
	code, err := NewGoppaCode(params, random)
	if err != nil {
		return nil, nil, err
	}

	scrambler, scramblerInverse, err := randomInvertibleMatrix(params.K, random)
	if err != nil {
		return nil, nil, err
	}
	// G1 = S*G: строки G складываются по строкам S.
	public, err := scrambler.Mul(code.Matrix)
	if err != nil {
		return nil, nil, err
	}

	return &PublicKey{Params: params, Matrix: public},
		&PrivateKey{Params: params, G: code.G, Support: code.Support, SInverse: scramblerInverse},
		nil
}

// GenerateKeys создаёт ключевую пару и возвращает её в двоичном формате.
func GenerateKeys(params Params) (public, private []byte, err error) {
	publicKey, privateKey, err := GenerateKeyPair(params, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return publicKey.MarshalBinary(), privateKey.MarshalBinary(), nil
}

// Encrypt зашифровывает сообщение случайным вектором ошибок.
func (k *PublicKey) Encrypt(message []byte) ([]byte, error) {
	return k.EncryptWithRandom(message, rand.Reader)
}

// EncryptWithRandom — зашифрование с явным источником случайности.
func (k *PublicKey) EncryptWithRandom(message []byte, random io.Reader) ([]byte, error) {
	params := k.Params
	if k.Matrix == nil || k.Matrix.Rows != params.K || k.Matrix.Cols != params.N {
		return nil, ErrBadPublicKey
	}
	if len(message) > (1<<32-1)/8 {
		return nil, fmt.Errorf("mceliece: message is longer than 512 MiB")
	}

	framed, totalBits := frameMessage(message)
	// Блоки по k бит; последний дополняется нулями. Пустое сообщение
	// всё равно даёт один блок: кадр содержит 32-битную длину.
	blocks := (totalBits + params.K - 1) / params.K
	blockBytes := params.N / 8
	cipher := make([]byte, blocks*blockBytes)

	for block := 0; block < blocks; block++ {
		messageBlock := newVector(params.K)
		for bit := 0; bit < params.K; bit++ {
			source := block*params.K + bit
			if source < totalBits && vectorBit(framed, source) {
				setVectorBit(messageBlock, bit)
			}
		}

		codeword := k.Matrix.MulVector(messageBlock)
		errors, err := randomErrorVector(params.N, params.T, random)
		if err != nil {
			return nil, err
		}
		for i := range codeword {
			codeword[i] ^= errors[i]
		}
		copy(cipher[block*blockBytes:], bitsToBytes(codeword, params.N))
	}
	return cipher, nil
}

// Decrypt расшифровывает сообщение закрытым ключом.
func (k *PrivateKey) Decrypt(cipher []byte) ([]byte, error) {
	params := k.Params
	blockBytes := params.N / 8
	if len(cipher) == 0 || len(cipher)%blockBytes != 0 {
		return nil, ErrBadCipher
	}
	if k.SInverse == nil || k.SInverse.Rows != params.K || k.SInverse.Cols != params.K {
		return nil, ErrBadPrivateKey
	}
	field, err := NewField(params.M)
	if err != nil {
		return nil, err
	}

	decoder := &Decoder{Field: field, Params: params, G: k.G, Support: k.Support}
	blocks := len(cipher) / blockBytes
	combined := newVector(blocks * params.K)

	for block := 0; block < blocks; block++ {
		received := bitsFromBytes(cipher[block*blockBytes:(block+1)*blockBytes], params.N)
		corrected, err := decoder.Decode(received)
		if err != nil {
			return nil, fmt.Errorf("%w: block %d: %v", ErrDecode, block, err)
		}
		// Первые k бит кодового слова — это M*S; умножаем на S^{-1}.
		message := k.SInverse.MulVector(corrected)
		for bit := 0; bit < params.K; bit++ {
			if vectorBit(message, bit) {
				setVectorBit(combined, block*params.K+bit)
			}
		}
	}

	return unframeMessage(combined, blocks, params)
}

// frameMessage добавляет к сообщению 32-битную длину в битах. Бит 0 —
// старший бит первого байта сообщения.
func frameMessage(message []byte) ([]uint64, int) {
	totalBits := framingBits + 8*len(message)
	framed := newVector(totalBits)

	length := uint32(8 * len(message))
	for bit := 0; bit < framingBits; bit++ {
		if length&(1<<uint(framingBits-1-bit)) != 0 {
			setVectorBit(framed, bit)
		}
	}
	for bit := 0; bit < 8*len(message); bit++ {
		if message[bit/8]&(0x80>>uint(bit%8)) != 0 {
			setVectorBit(framed, framingBits+bit)
		}
	}
	return framed, totalBits
}

// unframeMessage проверяет длину и нулевое дополнение последнего блока.
func unframeMessage(bits []uint64, blocks int, params Params) ([]byte, error) {
	if blocks*params.K < framingBits {
		return nil, ErrBadCipher
	}

	var length uint32
	for bit := 0; bit < framingBits; bit++ {
		length = length<<1 | boolToBit(vectorBit(bits, bit))
	}
	if length%8 != 0 || uint64(length) > uint64(blocks*params.K-framingBits) {
		return nil, ErrBadCipher
	}

	totalBits := framingBits + int(length)
	if (totalBits+params.K-1)/params.K != blocks {
		return nil, ErrBadCipher
	}
	for bit := totalBits; bit < blocks*params.K; bit++ {
		if vectorBit(bits, bit) {
			return nil, ErrBadCipher
		}
	}

	message := make([]byte, length/8)
	for i := range message {
		for bit := 0; bit < 8; bit++ {
			if vectorBit(bits, framingBits+i*8+bit) {
				message[i] |= 0x80 >> uint(bit)
			}
		}
	}
	return message, nil
}

func boolToBit(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

func randomInvertibleMatrix(size int, random io.Reader) (*Matrix, *Matrix, error) {
	for attempt := 0; attempt < 64; attempt++ {
		matrix, err := randomMatrix(size, size, random)
		if err != nil {
			return nil, nil, err
		}
		if inverse, ok := matrix.Invert(); ok {
			return matrix, inverse, nil
		}
	}
	return nil, nil, fmt.Errorf("mceliece: could not generate an invertible matrix")
}

func randomMatrix(rows, cols int, random io.Reader) (*Matrix, error) {
	matrix := NewMatrix(rows, cols)
	bytesNeeded := (rows*cols + 7) / 8
	data := make([]byte, bytesNeeded)
	if _, err := io.ReadFull(random, data); err != nil {
		return nil, err
	}
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			bit := i*cols + j
			if data[bit/8]&(0x80>>uint(bit%8)) != 0 {
				matrix.Set(i, j, true)
			}
		}
	}
	return matrix, nil
}

// randomErrorVector выбирает ровно weight различных позиций из n.
func randomErrorVector(n, weight int, random io.Reader) ([]uint64, error) {
	if weight > n {
		return nil, fmt.Errorf("mceliece: error weight %d exceeds block length %d", weight, n)
	}
	vector := newVector(n)
	positions := make([]int, n)
	for i := range positions {
		positions[i] = i
	}
	for i := 0; i < weight; i++ {
		offset, err := randomBelow(random, n-i)
		if err != nil {
			return nil, err
		}
		j := i + offset
		positions[i], positions[j] = positions[j], positions[i]
		setVectorBit(vector, positions[i])
	}
	return vector, nil
}
