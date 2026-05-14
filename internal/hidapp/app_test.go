package hidapp_test

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/bluez"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidapp"
)

type fakeDaemon struct {
	options bluez.DaemonOptions
	err     error
	called  bool
}

func (daemon *fakeDaemon) Run(_ context.Context, options bluez.DaemonOptions) error {
	daemon.called = true
	daemon.options = options

	return daemon.err
}

func TestHIDCLIはdaemonで設定からBLEkeyboardを起動する(t *testing.T) {
	configPath := writeConfig(t)
	daemon := &fakeDaemon{}

	code := hidapp.App{
		Daemon: daemon,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"daemon", "--config", configPath, "--test-text", "a"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if daemon.options.Adapter != "hci1" {
		t.Fatalf("adapter = %q, want hci1", daemon.options.Adapter)
	}
	if daemon.options.Name != "Desk Bridge" {
		t.Fatalf("name = %q, want Desk Bridge", daemon.options.Name)
	}
	if daemon.options.Appearance != bluez.KeyboardAppearance {
		t.Fatalf("appearance = %#v, want %#v", daemon.options.Appearance, bluez.KeyboardAppearance)
	}
	if !daemon.options.Pairable {
		t.Fatal("pairable = false, want true")
	}
	if !daemon.options.Discoverable {
		t.Fatal("discoverable = false, want true")
	}
	wantReports := [][]byte{
		{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	if !reflect.DeepEqual(daemon.options.TestReports, wantReports) {
		t.Fatalf("reports = %#v, want %#v", daemon.options.TestReports, wantReports)
	}
	if daemon.options.OnPeerReady == nil {
		t.Fatal("OnPeerReady is nil")
	}
}

func TestHIDCLIはinspectで実際に使うBLE設定を出す(t *testing.T) {
	configPath := writeConfig(t)
	stdout := &bytes.Buffer{}

	code := hidapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "inspect"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	for _, want := range []string{
		"adapter: hci1\n",
		"name: Desk Bridge\n",
		"appearance: keyboard (0x03C1)\n",
		"service_uuid: " + bluez.HIDServiceUUID + "\n",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Fatalf("stdout = %q, want to contain %q", stdout.String(), want)
		}
	}
}

func TestHIDCLIはBluetooth疎通後にtargetを設定へ保存する(t *testing.T) {
	configPath := writeConfig(t)
	daemon := &fakeDaemon{}

	code := hidapp.App{
		Daemon: daemon,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "daemon"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if err := daemon.options.OnPeerReady(bluez.Peer{
		Name:         "Work Laptop",
		BluetoothMAC: "AA:BB:CC:DD:EE:02",
	}); err != nil {
		t.Fatalf("OnPeerReady err = %v, want nil", err)
	}

	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Targets["work-laptop"]
	want := config.Target{Name: "Work Laptop", BluetoothMAC: "AA:BB:CC:DD:EE:02"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("target = %#v, want %#v", got, want)
	}
}

func writeConfig(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
behavior:
  disconnect_others: true
  reconnect_wait_sec: 0
hid:
  adapter: hci1
  name: Desk Bridge
  appearance: keyboard
  pairable: true
  discoverable: true
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}
