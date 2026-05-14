package localapp_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/localapp"
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

func TestPC側CLIはtarget名をRaspberryPi側へ渡す(t *testing.T) {
	configPath := writeLocalConfig(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "pc1"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "ssh",
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "switch", "pc1"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIはswitchサブコマンドでコマンドと同じtarget名を選べる(t *testing.T) {
	configPath := writeLocalConfigWithReservedName(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "switch", "status"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "ssh",
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "switch", "status"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIは不正なtarget名ではSSHを実行しない(t *testing.T) {
	configPath := writeLocalConfig(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "bad/name"})

	if code != 2 {
		t.Fatalf("終了コード = %d, want 2", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestPC側CLIはtarget一覧をRaspberryPi側へ問い合わせる(t *testing.T) {
	configPath := writeLocalConfig(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "list"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "ssh",
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "list"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIはtarget補完をRaspberryPi側へ問い合わせる(t *testing.T) {
	configPath := writeLocalConfigWithReservedName(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "__complete-targets"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "ssh",
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "__complete-targets"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIは環境変数の設定ファイルを読む(t *testing.T) {
	configPath := writeLocalConfig(t)
	t.Setenv("KBD_CONFIG", configPath)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"list"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := []commandCall{
		{
			name: "ssh",
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "list"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIは設定ファイルなしで補完スクリプトを出す(t *testing.T) {
	stdout := &bytes.Buffer{}

	code := localapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"completion", "zsh"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Fatal("stdout is empty")
	}
}

func writeLocalConfig(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func writeLocalConfigWithReservedName(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}
