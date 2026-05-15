//go:build linux

package input

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	eventSync = 0x00
	eventKey  = 0x01

	syncDropped = 0x03

	// EVIOCGRAB prevents forwarded key events from also reaching the Raspberry Pi console.
	evIOGrab = 0x40044590
)

type Forwarder struct {
	Paths []string
	Log   io.Writer
}

type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

func (forwarder Forwarder) Run(ctx context.Context, send func([]byte) error) error {
	paths, err := inputDevicePaths(forwarder.Paths)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		logf(forwarder.Log, "No keyboard input devices found at %s; set hid.input_devices to forward USB keyboard input\n", DefaultKeyboardGlob)
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(paths))
	for _, path := range paths {
		path := path
		go func() {
			errs <- readDevice(ctx, path, send)
		}()
	}

	for range paths {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errs:
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func inputDevicePaths(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = []string{DefaultKeyboardGlob}
	}

	seen := map[string]bool{}
	paths := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, "*?[") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("expand input device path %q: %w", pattern, err)
			}
			sort.Strings(matches)
			for _, match := range matches {
				if !seen[match] {
					seen[match] = true
					paths = append(paths, match)
				}
			}
			continue
		}

		if !seen[pattern] {
			seen[pattern] = true
			paths = append(paths, pattern)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func readDevice(ctx context.Context, path string, send func([]byte) error) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open input device %s: %w", path, err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()
	if err := unix.IoctlSetPointerInt(fd, evIOGrab, 1); err != nil {
		return fmt.Errorf("grab input device %s: %w", path, err)
	}
	defer func() {
		_ = unix.IoctlSetPointerInt(fd, evIOGrab, 0)
	}()

	var event inputEvent
	eventSize := int(unsafe.Sizeof(event))
	typeOffset := uintptr(unsafe.Offsetof(event.Type))
	codeOffset := uintptr(unsafe.Offsetof(event.Code))
	valueOffset := uintptr(unsafe.Offsetof(event.Value))

	buffer := make([]byte, eventSize*32)
	state := KeyboardState{}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		ready, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}, 250)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return fmt.Errorf("poll input device %s: %w", path, err)
		}
		if ready == 0 {
			continue
		}

		n, err := unix.Read(fd, buffer)
		if err == unix.EINTR || err == unix.EAGAIN {
			continue
		}
		if err != nil {
			return fmt.Errorf("read input device %s: %w", path, err)
		}
		if n == 0 {
			return fmt.Errorf("input device %s closed", path)
		}

		for offset := 0; offset+eventSize <= n; offset += eventSize {
			record := buffer[offset : offset+eventSize]
			eventType := binary.NativeEndian.Uint16(record[typeOffset:])
			eventCode := binary.NativeEndian.Uint16(record[codeOffset:])
			eventValue := int32(binary.NativeEndian.Uint32(record[valueOffset:]))

			switch eventType {
			case eventKey:
				report, changed := state.Apply(eventCode, eventValue)
				if changed {
					if err := send(report.Bytes()); err != nil {
						return err
					}
				}
			case eventSync:
				if eventCode == syncDropped {
					report, changed := state.Reset()
					if changed {
						if err := send(report.Bytes()); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}

func logf(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}

	_, _ = fmt.Fprintf(writer, format, args...)
}
