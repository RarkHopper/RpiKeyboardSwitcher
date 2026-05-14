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
`)

	if _, err := config.LoadLocal(path); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestPC側設定はtarget一覧なしを許可する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
`)

	if _, err := config.LoadLocal(path); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestPC側設定は接続先情報を拒否する(t *testing.T) {
	path := writeConfig(t, `
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
targets:
  laptop:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
`)

	if _, err := config.LoadLocal(path); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestRaspberryPi側設定はコマンドと同じtarget名を許可する(t *testing.T) {
	path := writeConfig(t, `
targets:
  switch:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:01
`)

	if _, err := config.LoadRPI(path); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestRaspberryPi側設定はtargetの不正なMACを拒否する(t *testing.T) {
	path := writeConfig(t, `
targets:
  laptop:
    name: Laptop
    bluetooth_mac: aa:bb:cc:dd:ee:02
`)

	if _, err := config.LoadRPI(path); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestRaspberryPi側設定はHID設定の既定値を補う(t *testing.T) {
	path := writeConfig(t, `{}`)

	cfg, err := config.LoadRPI(path)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if cfg.HID.Adapter != config.DefaultHIDAdapter {
		t.Fatalf("adapter = %q, want %q", cfg.HID.Adapter, config.DefaultHIDAdapter)
	}
	if cfg.HID.Name != config.DefaultHIDName {
		t.Fatalf("name = %q, want %q", cfg.HID.Name, config.DefaultHIDName)
	}
	if cfg.HID.Appearance != config.HIDAppearanceKeyboard {
		t.Fatalf("appearance = %q, want %q", cfg.HID.Appearance, config.HIDAppearanceKeyboard)
	}
	if !cfg.HID.PairableEnabled() {
		t.Fatal("pairable = false, want true")
	}
	if !cfg.HID.DiscoverableEnabled() {
		t.Fatal("discoverable = false, want true")
	}
}

func TestRaspberryPi側設定はHIDの不明なappearanceを拒否する(t *testing.T) {
	path := writeConfig(t, `
targets:
  laptop:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
hid:
  appearance: mouse
`)

	if _, err := config.LoadRPI(path); err == nil {
		t.Fatal("err = nil, want error")
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
