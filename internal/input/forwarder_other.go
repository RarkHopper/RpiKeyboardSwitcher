//go:build !linux

package input

import (
	"context"
	"io"
)

type Forwarder struct {
	Device string
	Log    io.Writer
}

func (forwarder Forwarder) Descriptor() (Descriptor, error) {
	return Descriptor{}, nil
}

func (forwarder Forwarder) Run(_ context.Context, _ func(Report) error) error {
	return nil
}
