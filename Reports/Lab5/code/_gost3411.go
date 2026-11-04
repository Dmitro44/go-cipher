package hash

// Хеш-функция ГОСТ Р 34.11-2012 («Стрибог»).
//
// Реализация следует RFC 6986. Состояние хеш-функции — вектор из 512 бит,
// то есть 64 байта. Внутри он хранится «младшим байтом вперёд»: элемент
// state[i] соответствует a_i из записи a_63 || ... || a_0. Поэтому при
// выводе дайджеста и при разборе эталонных примеров байты переставляются
// в обратном порядке: в стандарте вектор записывается старшим байтом вперёд.

import (
	"encoding/binary"
	"encoding/hex"
	"hash"
)

const (
	gostBlockSize = 64
	gost512Size   = 64
	gost256Size   = 32
)

// Pi — нелинейная подстановка Pi' из RFC 6986, п. 6.2. Каждый байт вектора
// заменяется по таблице: S(a) = Pi(a_63) || ... || Pi(a_0).
var pi = [256]byte{
	252, 238, 221, 17, 207, 110, 49, 22, 251, 196, 250,
	218, 35, 197, 4, 77, 233, 119, 240, 219, 147, 46,
	153, 186, 23, 54, 241, 187, 20, 205, 95, 193, 249,
	24, 101, 90, 226, 92, 239, 33, 129, 28, 60, 66,
	139, 1, 142, 79, 5, 132, 2, 174, 227, 106, 143,
	160, 6, 11, 237, 152, 127, 212, 211, 31, 235, 52,
	44, 81, 234, 200, 72, 171, 242, 42, 104, 162, 253,
	58, 206, 204, 181, 112, 14, 86, 8, 12, 118, 18,
	191, 114, 19, 71, 156, 183, 93, 135, 21, 161, 150,
	41, 16, 123, 154, 199, 243, 145, 120, 111, 157, 158,
	178, 177, 50, 117, 25, 61, 255, 53, 138, 126, 109,
	84, 198, 128, 195, 189, 13, 87, 223, 245, 36, 169,
	62, 168, 67, 201, 215, 121, 214, 246, 124, 34, 185,
	3, 224, 15, 236, 222, 122, 148, 176, 188, 220, 232,
	40, 80, 78, 51, 10, 74, 167, 151, 96, 115, 30,
	0, 98, 68, 26, 184, 56, 130, 100, 159, 38, 65,
	173, 69, 70, 146, 39, 94, 85, 47, 140, 163, 165,
	125, 105, 213, 149, 59, 7, 88, 179, 64, 134, 172,
	29, 247, 48, 55, 107, 228, 136, 217, 231, 137, 225,
	27, 131, 73, 76, 63, 248, 254, 141, 83, 170, 144,
	202, 216, 133, 97, 32, 113, 103, 164, 45, 43, 9,
	91, 203, 155, 37, 208, 190, 229, 108, 82, 89, 166,
	116, 210, 230, 244, 180, 192, 209, 102, 175, 194, 57,
	75, 99, 182,
}

// Tau — перестановка байтов из RFC 6986, п. 6.3:
// P(a) = a_{Tau(63)} || ... || a_{Tau(0)}.
var tau = [64]byte{
	0, 8, 16, 24, 32, 40, 48, 56,
	1, 9, 17, 25, 33, 41, 49, 57,
	2, 10, 18, 26, 34, 42, 50, 58,
	3, 11, 19, 27, 35, 43, 51, 59,
	4, 12, 20, 28, 36, 44, 52, 60,
	5, 13, 21, 29, 37, 45, 53, 61,
	6, 14, 22, 30, 38, 46, 54, 62,
	7, 15, 23, 31, 39, 47, 55, 63,
}

// aMatrix — строки матрицы A линейного преобразования l (RFC 6986, п. 6.4).
// Строка j используется при умножении на бит b_i, где i = 63 - j.
var aMatrix = [64]uint64{
	0x8e20faa72ba0b470, 0x47107ddd9b505a38, 0xad08b0e0c3282d1c, 0xd8045870ef14980e,
	0x6c022c38f90a4c07, 0x3601161cf205268d, 0x1b8e0b0e798c13c8, 0x83478b07b2468764,
	0xa011d380818e8f40, 0x5086e740ce47c920, 0x2843fd2067adea10, 0x14aff010bdd87508,
	0x0ad97808d06cb404, 0x05e23c0468365a02, 0x8c711e02341b2d01, 0x46b60f011a83988e,
	0x90dab52a387ae76f, 0x486dd4151c3dfdb9, 0x24b86a840e90f0d2, 0x125c354207487869,
	0x092e94218d243cba, 0x8a174a9ec8121e5d, 0x4585254f64090fa0, 0xaccc9ca9328a8950,
	0x9d4df05d5f661451, 0xc0a878a0a1330aa6, 0x60543c50de970553, 0x302a1e286fc58ca7,
	0x18150f14b9ec46dd, 0x0c84890ad27623e0, 0x0642ca05693b9f70, 0x0321658cba93c138,
	0x86275df09ce8aaa8, 0x439da0784e745554, 0xafc0503c273aa42a, 0xd960281e9d1d5215,
	0xe230140fc0802984, 0x71180a8960409a42, 0xb60c05ca30204d21, 0x5b068c651810a89e,
	0x456c34887a3805b9, 0xac361a443d1c8cd2, 0x561b0d22900e4669, 0x2b838811480723ba,
	0x9bcf4486248d9f5d, 0xc3e9224312c8c1a0, 0xeffa11af0964ee50, 0xf97d86d98a327728,
	0xe4fa2054a80b329c, 0x727d102a548b194e, 0x39b008152acb8227, 0x9258048415eb419d,
	0x492c024284fbaec0, 0xaa16012142f35760, 0x550b8e9e21f7a530, 0xa48b474f9ef5dc18,
	0x70a6a56e2440598e, 0x3853dc371220a247, 0x1ca76e95091051ad, 0x0edd37c48a08a6d8,
	0x07e095624504536c, 0x8d70c431ac02a736, 0xc83862965601dd1b, 0x641c314b2b8ee083,
}

// iterationConstantsHex — константы итерации C[1]..C[12] из RFC 6986, п. 6.5,
// записанные в порядке стандарта (старшим байтом вперёд). В init они
// переворачиваются в рабочее представление состояния.
var iterationConstantsHex = [12]string{
	"b1085bda1ecadae9ebcb2f81c0657c1f2f6a76432e45d016714eb88d7585c4fc4b7ce09192676901a2422a08a460d31505767436cc744d23dd806559f2a64507",
	"6fa3b58aa99d2f1a4fe39d460f70b5d7f3feea720a232b9861d55e0f16b501319ab5176b12d699585cb561c2db0aa7ca55dda21bd7cbcd56e679047021b19bb7",
	"f574dcac2bce2fc70a39fc286a3d843506f15e5f529c1f8bf2ea7514b1297b7bd3e20fe490359eb1c1c93a376062db09c2b6f443867adb31991e96f50aba0ab2",
	"ef1fdfb3e81566d2f948e1a05d71e4dd488e857e335c3c7d9d721cad685e353fa9d72c82ed03d675d8b71333935203be3453eaa193e837f1220cbebc84e3d12e",
	"4bea6bacad4747999a3f410c6ca923637f151c1f1686104a359e35d7800fffbdbfcd1747253af5a3dfff00b723271a167a56a27ea9ea63f5601758fd7c6cfe57",
	"ae4faeae1d3ad3d96fa4c33b7a3039c02d66c4f95142a46c187f9ab49af08ec6cffaa6b71c9ab7b40af21f66c2bec6b6bf71c57236904f35fa68407a46647d6e",
	"f4c70e16eeaac5ec51ac86febf240954399ec6c7e6bf87c9d3473e33197a93c90992abc52d822c3706476983284a05043517454ca23c4af38886564d3a14d493",
	"9b1f5b424d93c9a703e7aa020c6e41414eb7f8719c36de1e89b4443b4ddbc49af4892bcb929b069069d18d2bd1a5c42f36acc2355951a8d9a47f0dd4bf02e71e",
	"378f5a541631229b944c9ad8ec165fde3a7d3a1b258942243cd955b7e00d0984800a440bdbb2ceb17b2b8a9aa6079c540e38dc92cb1f2a607261445183235adb",
	"abbedea680056f52382ae548b2e4f3f38941e71cff8a78db1fffe18a1b3361039fe76702af69334b7a1e6c303b7652f43698fad1153bb6c374b4c7fb98459ced",
	"7bcd9ed0efc889fb3002c6cd635afe94d8fa6bbbebab076120018021148466798a1d71efea48b9caefbacd1d7d476e98dea2594ac06fd85d6bcaa4cd81f32d1b",
	"378ee767f11631bad21380b00449b17acda43c32bcdf1d77f82012d430219f9b5d80ef9d1891cc86e71da4aa88e12852faf417d5d9b21b9948bc924af11bd720",
}

// iterationConstants — те же константы в рабочем порядке байтов.
var iterationConstants [12][64]byte

func init() {
	for i, s := range iterationConstantsHex {
		raw, err := hex.DecodeString(s)
		if err != nil {
			panic("gost3411: некорректная константа C[%d]: " + err.Error())
		}
		iterationConstants[i] = stateFromBigEndian(raw)
	}
}

// stateFromBigEndian переставляет байты из записи стандарта (a_63 || ... || a_0)
// во внутреннее представление (a_0 в нулевом элементе).
func stateFromBigEndian(b []byte) [64]byte {
	var s [64]byte
	for i := 0; i < 64; i++ {
		s[i] = b[63-i]
	}
	return s
}

// blockFromMessage записывает 64 байта сообщения в состояние: первый байт
// блока становится младшим, как требует порядок обработки стандарта.
func blockFromMessage(b []byte) [64]byte {
	var s [64]byte
	copy(s[:], b)
	return s
}

// S — нелинейная подстановка: каждый байт вектора заменяется по таблице Pi.
func S(a [64]byte) [64]byte {
	var out [64]byte
	for i := 0; i < 64; i++ {
		out[i] = pi[a[i]]
	}
	return out
}

// P — перестановка байтов: новый байт i равен байту Tau(i) исходного вектора.
func P(a [64]byte) [64]byte {
	var out [64]byte
	for i := 0; i < 64; i++ {
		out[i] = a[tau[i]]
	}
	return out
}

// L — линейное преобразование. Вектор делится на восемь 64-битных частей,
// каждая умножается на матрицу A над GF(2):
//
//	c = b_63 * A[0] xor ... xor b_0 * A[63].
func L(a [64]byte) [64]byte {
	var out [64]byte
	for part := 0; part < 8; part++ {
		b := binary.LittleEndian.Uint64(a[part*8 : part*8+8])
		var c uint64
		for bit := 0; bit < 64; bit++ {
			if b&(1<<uint(bit)) != 0 {
				c ^= aMatrix[63-bit]
			}
		}
		binary.LittleEndian.PutUint64(out[part*8:part*8+8], c)
	}
	return out
}

// lps — композиция L(P(S(a))), основная перестановка раунда.
func lps(a [64]byte) [64]byte {
	return L(P(S(a)))
}

// X — сложение по модулю 2 (побитовое исключающее ИЛИ) двух векторов.
func X(a, b [64]byte) [64]byte {
	var out [64]byte
	for i := 0; i < 64; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// keySchedule вычисляет K[i+1] = LPS(K[i] xor C[i]) для раунда i (1..12).
func keySchedule(k [64]byte, round int) [64]byte {
	return lps(X(k, iterationConstants[round-1]))
}

// E — раундовая функция: 12 раз применяет LPS и складывает результат
// с очередным ключом K[i], начиная с X[K[1]](m).
func E(k, m [64]byte) [64]byte {
	state := X(k, m)
	for round := 1; round <= 12; round++ {
		state = lps(state)
		k = keySchedule(k, round)
		state = X(state, k)
	}
	return state
}

// gN — функция сжатия g_N(h, m) = E(LPS(h xor N), m) xor h xor m.
func gN(h, m, n [64]byte) [64]byte {
	k := lps(X(h, n))
	t := E(k, m)
	return X(X(t, h), m)
}

// add512 складывает два 512-битных вектора по модулю 2^512.
func add512(a, b [64]byte) [64]byte {
	var out [64]byte
	carry := 0
	for i := 0; i < 64; i++ {
		sum := int(a[i]) + int(b[i]) + carry
		out[i] = byte(sum)
		carry = sum >> 8
	}
	return out
}

// addBits прибавляет к вектору число битов (N := N + |M| mod 2^512).
func addBits(a [64]byte, bits uint64) [64]byte {
	var b [64]byte
	binary.LittleEndian.PutUint64(b[:8], bits)
	return add512(a, b)
}

// gost3411 — реализация hash.Hash для ГОСТ Р 34.11-2012.
//
// Принятые через Write байты накапливаются: Sum вычисляет дайджест,
// не изменяя состояние.
type gost3411 struct {
	size    int
	iv      [64]byte
	message []byte
}

// NewGOST3411_512 возвращает хеш-функцию с 512-битным кодом (IV = 0^512).
func NewGOST3411_512() hash.Hash {
	return &gost3411{size: gost512Size}
}

// NewGOST3411_256 возвращает хеш-функцию с 256-битным кодом (IV = (00000001)^64).
func NewGOST3411_256() hash.Hash {
	var iv [64]byte
	for i := range iv {
		iv[i] = 0x01
	}
	return &gost3411{size: gost256Size, iv: iv}
}

func (d *gost3411) Size() int      { return d.size }
func (d *gost3411) BlockSize() int { return gostBlockSize }

// Reset сбрасывает накопленное сообщение; начальное значение h не меняется.
func (d *gost3411) Reset() {
	d.message = d.message[:0]
}

// Write добавляет данные к сообщению. Ошибок быть не может.
func (d *gost3411) Write(p []byte) (int, error) {
	d.message = append(d.message, p...)
	return len(p), nil
}

// Sum дописывает к b дайджест, не изменяя состояние.
func (d *gost3411) Sum(b []byte) []byte {
	h := d.finish()
	// Дайджест выдаётся старшим байтом вперёд. Для 256-битного варианта
	// берётся MSB_256 — старшие 32 байта состояния.
	for i := 0; i < d.size; i++ {
		b = append(b, h[63-i])
	}
	return b
}

// finish выполняет шаги 2 и 3 стандарта: обработку полных блоков с конца,
// дополнение 0...01|M, затем две заключительные функции g_0.
func (d *gost3411) finish() [64]byte {
	h := d.iv
	var n, sigma [64]byte
	data := d.message

	// Шаг 2: пока в сообщении есть полный блок, берём очередные 512 бит.
	for len(data) >= gostBlockSize {
		block := blockFromMessage(data[:gostBlockSize])
		h = gN(h, block, n)
		n = addBits(n, 512)
		sigma = add512(sigma, block)
		data = data[gostBlockSize:]
	}

	// Шаг 3.1: m = 0^(511-|M|) || 1 || M.
	var last [64]byte
	copy(last[:], data)
	last[len(data)] = 0x01

	// Шаги 3.2–3.4.
	h = gN(h, last, n)
	n = addBits(n, uint64(len(data))*8)
	sigma = add512(sigma, last)

	// Шаги 3.5–3.6: h = g_0(h, N), h = g_0(h, Sigma).
	zero := [64]byte{}
	h = gN(h, n, zero)
	h = gN(h, sigma, zero)
	return h
}
