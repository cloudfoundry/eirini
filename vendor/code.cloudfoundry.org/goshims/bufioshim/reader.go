package bufioshim

//go:generate counterfeiter -o bufio_fake/fake_reader.go . Reader
type Reader interface {
	ReadString(byte) (string, error)
}
