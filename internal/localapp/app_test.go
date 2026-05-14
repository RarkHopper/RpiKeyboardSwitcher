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

func TestPC側CLIはtargetをSSH呼び出しへ変換する(t *testing.T) {
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
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "switch", "desktop"},
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
			args: []string{"pi@rpi-kbd.local", "kbd-rpi", "switch", "switch"},
		},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestPC側CLIは未知のtargetではSSHを実行しない(t *testing.T) {
	configPath := writeLocalConfig(t)
	runner := &fakeRunner{}

	code := localapp.App{
		Runner: runner,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "missing"})

	if code != 2 {
		t.Fatalf("終了コード = %d, want 2", code)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestPC側CLIはローカルtarget一覧を表示する(t *testing.T) {
	configPath := writeLocalConfig(t)
	stdout := &bytes.Buffer{}

	code := localapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "list"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := "pc1 -> desktop\npc2 -> laptop\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestPC側CLIは設定済みtarget名を補完候補として出す(t *testing.T) {
	configPath := writeLocalConfigWithReservedName(t)
	stdout := &bytes.Buffer{}

	code := localapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "__complete-targets"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := "pc1\nstatus\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestPC側CLIは環境変数の設定ファイルを読む(t *testing.T) {
	configPath := writeLocalConfig(t)
	t.Setenv("KBD_CONFIG", configPath)
	stdout := &bytes.Buffer{}

	code := localapp.App{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"list"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}

	want := "pc1 -> desktop\npc2 -> laptop\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
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

targets:
  pc2: laptop
  pc1: desktop
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

targets:
  pc1: desktop
  status: switch
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}
