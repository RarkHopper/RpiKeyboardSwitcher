package rpiapp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/rpiapp"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/state"
)

type commandCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []commandCall
	err   error
}

func (runner *fakeRunner) Run(_ context.Context, _ io.Reader, _ io.Writer, _ io.Writer, name string, args ...string) error {
	runner.calls = append(runner.calls, commandCall{
		name: name,
		args: append([]string(nil), args...),
	})

	return runner.err
}

func TestRaspberryPi側CLIはswitchで接続してstateを書き込む(t *testing.T) {
	configPath := writeRPIConfig(t)
	statePath := t.TempDir() + "/state.json"
	runner := &fakeRunner{}
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)

	code := rpiapp.App{
		Runner: runner,
		Now:    func() time.Time { return now },
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "--state", statePath, "switch", "laptop"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	wantCalls := []commandCall{
		{
			name: "bluetoothctl",
			args: []string{"connect", "AA:BB:CC:DD:EE:02"},
		},
	}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, wantCalls)
	}

	got := readState(t, statePath)
	want := state.State{
		Target:       "laptop",
		BluetoothMAC: "AA:BB:CC:DD:EE:02",
		UpdatedAt:    now,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state = %#v, want %#v", got, want)
	}
}

func TestRaspberryPi側CLIはコマンドと同じtarget名を補完候補として出す(t *testing.T) {
	configPath := writeRPIConfigWithReservedName(t)
	stdout := &bytes.Buffer{}

	code := rpiapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "__complete-targets"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := "desktop\nswitch\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRaspberryPi側CLIは環境変数の設定ファイルを補完に使う(t *testing.T) {
	configPath := writeRPIConfig(t)
	t.Setenv("KBD_RPI_CONFIG", configPath)
	stdout := &bytes.Buffer{}

	code := rpiapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"__complete-targets"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := "desktop\nlaptop\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRaspberryPi側CLIは設定ファイルなしで補完スクリプトを出す(t *testing.T) {
	stdout := &bytes.Buffer{}

	code := rpiapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"completion", "bash"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}
}

func TestRaspberryPi側CLIは前回接続先が違うときに切断してから接続する(t *testing.T) {
	configPath := writeRPIConfig(t)
	statePath := t.TempDir() + "/state.json"
	writeState(t, statePath, state.State{
		Target:       "desktop",
		BluetoothMAC: "AA:BB:CC:DD:EE:01",
		UpdatedAt:    time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC),
	})
	runner := &fakeRunner{}

	code := rpiapp.App{
		Runner: runner,
		Now:    func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) },
		Sleep:  func(time.Duration) {},
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "--state", statePath, "switch", "laptop"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "bluetoothctl",
			args: []string{"disconnect", "AA:BB:CC:DD:EE:01"},
		},
		{
			name: "bluetoothctl",
			args: []string{"connect", "AA:BB:CC:DD:EE:02"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestRaspberryPi側CLIはdisconnectで切断してstateを削除する(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	writeState(t, statePath, state.State{
		Target:       "laptop",
		BluetoothMAC: "AA:BB:CC:DD:EE:02",
		UpdatedAt:    time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
	})
	runner := &fakeRunner{}

	code := rpiapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--state", statePath, "disconnect"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "bluetoothctl",
			args: []string{"disconnect", "AA:BB:CC:DD:EE:02"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state exists after disconnect: %v", err)
	}
}

func TestRaspberryPi側CLIは不正なMACを含む設定を拒否する(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	content := []byte(`
targets:
  laptop:
    name: Laptop
    bluetooth_mac: aa:bb:cc:dd:ee:02
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}

	code := rpiapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "--state", t.TempDir() + "/state.json", "switch", "laptop"})

	if code != 2 {
		t.Fatalf("終了コード = %d, want 2", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestRaspberryPi側CLIは未知のtargetでは接続しない(t *testing.T) {
	configPath := writeRPIConfig(t)
	runner := &fakeRunner{}

	code := rpiapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "--state", t.TempDir() + "/state.json", "switch", "missing"})

	if code != 2 {
		t.Fatalf("終了コード = %d, want 2", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestRaspberryPi側CLIは壊れたstateでは失敗する(t *testing.T) {
	configPath := writeRPIConfig(t)
	statePath := t.TempDir() + "/state.json"
	if err := os.WriteFile(statePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}

	code := rpiapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "--state", statePath, "switch", "laptop"})

	if code != 2 {
		t.Fatalf("終了コード = %d, want 2", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func writeRPIConfig(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
targets:
  desktop:
    name: Main Desktop
    bluetooth_mac: AA:BB:CC:DD:EE:01
  laptop:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
behavior:
  disconnect_others: true
  reconnect_wait_sec: 0
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func writeRPIConfigWithReservedName(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
targets:
  desktop:
    name: Main Desktop
    bluetooth_mac: AA:BB:CC:DD:EE:01
  switch:
    name: Switch Named Target
    bluetooth_mac: AA:BB:CC:DD:EE:03
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func writeState(t *testing.T, path string, current state.State) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err := json.NewEncoder(file).Encode(current); err != nil {
		t.Fatal(err)
	}
}

func readState(t *testing.T, path string) state.State {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = file.Close()
	}()

	var current state.State
	if err := json.NewDecoder(file).Decode(&current); err != nil {
		t.Fatal(err)
	}

	return current
}
