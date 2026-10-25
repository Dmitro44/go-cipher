package cipher

// KeyOption is one selectable key-generation preset. Bits is the
// algorithm-specific security parameter (modulus size, code length, ...).
type KeyOption struct {
	Label string
	Bits  int
}

// KeyPair is the asymmetric key material. Both halves are raw bytes; the UI
// renders them as hex.
type KeyPair struct {
	Public  []byte
	Private []byte
}

// AsymCipher is an asymmetric primitive driven by the UI. Each implementation
// lives in its own tab, so there is no method selector.
type AsymCipher interface {
	Name() string
	// KeyOptions returns the selectable key-generation presets.
	KeyOptions() []KeyOption
	GenerateKeys(option KeyOption) (KeyPair, error)
	Encrypt(pub, msg []byte) ([]byte, error)
	// Decrypt returns every plausible plaintext. Rabin yields four square
	// roots, so the pane presents them as candidates.
	Decrypt(priv, cipherText []byte) ([][]byte, error)
}
