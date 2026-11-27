package cipher

// SignatureScheme — примитив электронной цифровой подписи, которым
// пользуется панель подписи. Ключи и подпись передаются сырыми байтами,
// интерфейс показывает их в шестнадцатеричном виде.
type SignatureScheme interface {
	Name() string
	// KeyOptions возвращает доступные наборы параметров кривой.
	KeyOptions() []KeyOption
	// GenerateKeys формирует ключевую пару: public = x||y, private = d.
	GenerateKeys(option KeyOption) (KeyPair, error)
	// Sign возвращает подпись r||s, каждое число фиксированной ширины.
	Sign(priv, msg []byte) ([]byte, error)
	// Verify проверяет подпись r||s открытым ключом x||y.
	Verify(pub, msg, sig []byte) (bool, error)
}
