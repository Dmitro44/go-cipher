package cipher

type Cipher interface {
	Name() string
	Methods() []string
	KeySize() int
	Run(method int, decrypt bool, in, key []byte) (string, error)
}
