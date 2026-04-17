package none

import (
	"github.com/allape/openkvm/kvm/clipboard"
)

type Clipboard struct{}

func (c *Clipboard) Open() error {
	return nil
}

func (c *Clipboard) Close() error {
	return nil
}

func (c *Clipboard) Read(buffer []byte) (int, error) {
	return 0, nil
}

func (c *Clipboard) Write(buffer []byte) (int, error) {
	return len(buffer), nil
}

var _ clipboard.Driver = (*Clipboard)(nil)
