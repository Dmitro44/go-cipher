package mceliece

import "errors"

// Ошибки алгоритма. Сообщения точные: вызывающий код (адаптер и панель
// интерфейса) показывает их пользователю без переформулировки.
var (
	ErrNotInvertible = errors.New("mceliece: polynomial is not invertible modulo the Goppa polynomial")
	ErrDecode        = errors.New("mceliece: error correction failed")
	ErrBadPublicKey  = errors.New("mceliece: malformed public key")
	ErrBadPrivateKey = errors.New("mceliece: malformed private key")
	ErrBadCipher     = errors.New("mceliece: ciphertext length or framing is invalid")
	ErrUnsupported   = errors.New("mceliece: unsupported parameter set")
)
