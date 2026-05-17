//go:build linux

package input

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/sys/unix"
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

	buffer := make([]byte, 4096)
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
			return fmt.Errorf("poll hidraw device %s: %w", forwarder.Device, err)
		}
		if ready == 0 {
			continue
		}

		n, err := unix.Read(fd, buffer)
		if err == unix.EINTR || err == unix.EAGAIN {
			continue
		}
		if err != nil {
			return fmt.Errorf("read hidraw device %s: %w", forwarder.Device, err)
		}
		if n == 0 {
			return fmt.Errorf("hidraw device %s closed", forwarder.Device)
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
		return Descriptor{}, fmt.Errorf("hidraw device %s returned empty report descriptor", device)
	}

	raw := unix.HIDRawReportDescriptor{Size: uint32(size)}
	if err := unix.IoctlHIDGetDesc(fd, &raw); err != nil {
		return Descriptor{}, fmt.Errorf("read hidraw report descriptor %s: %w", device, err)
	}
	if int(raw.Size) > len(raw.Value) {
		return Descriptor{}, fmt.Errorf("hidraw report descriptor %s is too large: %d bytes", device, raw.Size)
	}

	return ParseDescriptor(raw.Value[:raw.Size])
}

func logf(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}

	_, _ = fmt.Fprintf(writer, format, args...)
}
