//go:build !linux

package input

import (
	"context"
	"errors"
	"io"
)

var ErrUnsupportedOS = errors.New("input forwarder is not supported on non-linux")

type Forwarder struct {
	Device string
	Log    io.Writer
}

func (Forwarder) Descriptor() (Descriptor, error) {
	return Descriptor{}, ErrUnsupportedOS
}

func (Forwarder) Run(_ context.Context, _ func(Report) error) error {
	return ErrUnsupportedOS
}
