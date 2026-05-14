package config_test

import (
	"os"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
)

func Test設定読み込みは未知フィールドを拒否する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
  port: 22
targets:
  pc1: desktop
`)

	if _, err := config.LoadLocal(path); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestPC側設定はremote_commandの空白を拒否する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: sudo kbd-rpi
targets:
  pc1: desktop
`)

	if _, err := config.LoadLocal(path); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestPC側設定はコマンドと同じtarget名を許可する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
targets:
  status: desktop
`)

	if _, err := config.LoadLocal(path); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestPC側設定はRaspberryPi側コマンドと同じ接続先名を許可する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
targets:
  pc1: disconnect
`)

	if _, err := config.LoadLocal(path); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestRaspberryPi側設定はコマンドと同じdevice名を許可する(t *testing.T) {
	path := writeConfig(t, `
devices:
  switch:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
`)

	if _, err := config.LoadRPI(path); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}
