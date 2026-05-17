//go:build linux

package input

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"golang.org/x/sys/unix"
)

var (
	errHIDRawClosed                   = errors.New("hidraw device closed")
	errEmptyHIDRawReportDescriptor    = errors.New("hidraw device returned empty report descriptor")
	errHIDRawReportDescriptorTooLarge = errors.New("hidraw report descriptor is too large")
	errFileDescriptorOutOfRange       = errors.New("hidraw file descriptor is out of range")
	errDescriptorSizeOutOfRange       = errors.New("hidraw report descriptor size is out of range")
)

type Forwarder struct {
	Device string
	Log    io.Writer
}

func (forwarder Forwarder) Descriptor() (Descriptor, error) {
	fd, err := unix.Open(forwarder.Device, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return Descriptor{}, fmt.Errorf("open hidraw device %s: %w", forwarder.Device, err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	return readDescriptor(fd, forwarder.Device)
}

func (forwarder Forwarder) Run(ctx context.Context, send func(Report) error) error {
	fd, err := unix.Open(forwarder.Device, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open hidraw device %s: %w", forwarder.Device, err)
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	descriptor, err := readDescriptor(fd, forwarder.Device)
	if err != nil {
		return err
	}
	logf(forwarder.Log, "Forwarding HID reports from %s with %d byte report descriptor\n", forwarder.Device, len(descriptor.ReportMap))

	pollFD, err := pollFileDescriptor(fd)
	if err != nil {
		return err
	}
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		ready, err := unix.Poll([]unix.PollFd{{Fd: pollFD, Events: unix.POLLIN}}, 250)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("poll hidraw device %s: %w", forwarder.Device, err)
		}
		if ready == 0 {
			continue
		}

		n, err := unix.Read(fd, buffer)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read hidraw device %s: %w", forwarder.Device, err)
		}
		if n == 0 {
			return fmt.Errorf("%w: %s", errHIDRawClosed, forwarder.Device)
		}

		report, ok := descriptor.Report(buffer[:n])
		if !ok {
			continue
		}
		if err := send(report); err != nil {
			return err
		}
	}
}

func readDescriptor(fd int, device string) (Descriptor, error) {
	size, err := unix.IoctlGetInt(fd, uint(unix.HIDIOCGRDESCSIZE))
	if err != nil {
		return Descriptor{}, fmt.Errorf("read hidraw descriptor size %s: %w", device, err)
	}
	if size <= 0 {
		return Descriptor{}, fmt.Errorf("%w: %s", errEmptyHIDRawReportDescriptor, device)
	}
	if size > math.MaxUint32 {
		return Descriptor{}, fmt.Errorf("%w: %s has %d bytes", errDescriptorSizeOutOfRange, device, size)
	}

	raw := unix.HIDRawReportDescriptor{Size: uint32(size)}
	if err := unix.IoctlHIDGetDesc(fd, &raw); err != nil {
		return Descriptor{}, fmt.Errorf("read hidraw report descriptor %s: %w", device, err)
	}
	if raw.Size > uint32(len(raw.Value)) {
		return Descriptor{}, fmt.Errorf("%w: %s has %d bytes", errHIDRawReportDescriptorTooLarge, device, raw.Size)
	}

	return ParseDescriptor(raw.Value[:raw.Size])
}

func pollFileDescriptor(fd int) (int32, error) {
	if fd < 0 || fd > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %d", errFileDescriptorOutOfRange, fd)
	}
	return int32(fd), nil
}

func logf(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}

	_, _ = fmt.Fprintf(writer, format, args...)
}
