package bufioshim

import (
	"bufio"
)

type ReaderShim struct {
	Delegate *bufio.Reader
}

func (r *ReaderShim) ReadString(delim byte) (string, error) {
	return r.Delegate.ReadString(delim)
}
