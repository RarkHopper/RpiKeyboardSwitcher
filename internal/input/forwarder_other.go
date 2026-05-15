//go:build !linux

package input

import (
	"context"
	"io"
)

type Forwarder struct {
	Paths []string
	Log   io.Writer
}

func (forwarder Forwarder) Run(_ context.Context, _ func([]byte) error) error {
	return nil
}
