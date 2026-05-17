package rpiapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/complete"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/execx"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/state"
)

var (
	errMissingTarget       = errors.New("missing target")
	errUnknownSwitchOption = errors.New("unknown switch option")
)

type App struct {
	Runner  execx.Runner
	Context context.Context
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Now     func() time.Time
	Sleep   func(time.Duration)
}

func (app App) Run(args []string) int {
	fs := flag.NewFlagSet("kbd-rpi", flag.ContinueOnError)
	fs.SetOutput(app.stderr())

	configPath := fs.String("config", "", "config file path")
	statePath := fs.String("state", config.DefaultStatePath, "state file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	operands := fs.Args()
	if len(operands) == 0 {
		_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi [--config path] [--state path] <switch|status|list|disconnect> [target]")
		return 2
	}

	switch operands[0] {
	case "__complete-targets":
		if len(operands) != 1 {
			return 2
		}
		path := resolveConfigPath(*configPath)

		return app.completeTargets(path)
	case "completion":
		if len(operands) != 2 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi completion <bash|zsh>")
			return 2
		}

		return printRPICompletion(app.stdout(), operands[1])
	case "switch":
		req, err := parseSwitchRequest(operands[1:])
		if err != nil {
			_, _ = fmt.Fprintln(app.stderr(), err)
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi [--config path] [--state path] switch <target>")
			return 2
		}

		path := resolveConfigPath(*configPath)

		return app.switchTarget(path, *statePath, req)
	case "status":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi [--config path] [--state path] status")
			return 2
		}

		return app.status(*statePath)
	case "list":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi [--config path] [--state path] list")
			return 2
		}

		path := resolveConfigPath(*configPath)

		return app.listTargets(path)
	case "disconnect":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-rpi [--config path] [--state path] disconnect")
			return 2
		}

		return app.disconnect(*statePath)
	default:
		_, _ = fmt.Fprintf(app.stderr(), "unknown command: %s\n", operands[0])
		return 2
	}
}

func resolveConfigPath(path string) string {
	if path != "" {
		return path
	}
	if envPath := os.Getenv("KBD_RPI_CONFIG"); envPath != "" {
		return envPath
	}

	return config.DefaultRPIConfigPath
}

func (app App) completeTargets(configPath string) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		return 2
	}

	complete.PrintValues(app.stdout(), complete.MapKeys(cfg.Targets))
	return 0
}

type switchRequest struct {
	target string
}

func parseSwitchRequest(args []string) (switchRequest, error) {
	if len(args) == 0 {
		return switchRequest{}, errMissingTarget
	}

	req := switchRequest{target: args[0]}
	if err := config.ValidateName("switch target", req.target); err != nil {
		return switchRequest{}, fmt.Errorf("validate switch target: %w", err)
	}
	if len(args) == 1 {
		return req, nil
	}

	return switchRequest{}, fmt.Errorf("%w: %s", errUnknownSwitchOption, args[1])
}

func (app App) switchTarget(configPath string, statePath string, req switchRequest) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	target, ok := cfg.Targets[req.target]
	if !ok {
		_, _ = fmt.Fprintf(app.stderr(), "unknown target: %s\n", req.target)
		return 2
	}

	current, ok, err := state.Load(statePath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	if ok && current.BluetoothMAC != target.BluetoothMAC && cfg.Behavior.ShouldDisconnectOthers() {
		if err := app.runBluetoothctl("disconnect", current.BluetoothMAC); err != nil {
			_, _ = fmt.Fprintln(app.stderr(), err)
			return 1
		}
		app.sleep(time.Duration(cfg.Behavior.ReconnectWaitSec) * time.Second)
	}

	if err := app.runBluetoothctl("connect", target.BluetoothMAC); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}

	next := state.State{
		Target:       req.target,
		BluetoothMAC: target.BluetoothMAC,
		UpdatedAt:    app.now().UTC(),
	}
	if err := state.Save(statePath, next); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}

	return 0
}

func (app App) status(statePath string) int {
	current, ok, err := state.Load(statePath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}
	if !ok {
		_, _ = fmt.Fprintln(app.stdout(), "current: none")
		return 0
	}

	_, _ = fmt.Fprintf(app.stdout(), "current: %s\nbluetooth_mac: %s\n", current.Target, current.BluetoothMAC)
	return 0
}

func (app App) listTargets(configPath string) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	keys := make([]string, 0, len(cfg.Targets))
	for key := range cfg.Targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		target := cfg.Targets[key]
		_, _ = fmt.Fprintf(app.stdout(), "%s -> %s (%s)\n", key, target.Name, target.BluetoothMAC)
	}

	return 0
}

func (app App) disconnect(statePath string) int {
	current, ok, err := state.Load(statePath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}
	if !ok {
		_, _ = fmt.Fprintln(app.stdout(), "current: none")
		return 0
	}

	if err := app.runBluetoothctl("disconnect", current.BluetoothMAC); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}
	if err := state.Remove(statePath); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}

	return 0
}

func printRPICompletion(stdout io.Writer, shell string) int {
	switch shell {
	case "bash":
		_, _ = fmt.Fprint(stdout, rpiBashCompletion)
		return 0
	case "zsh":
		_, _ = fmt.Fprint(stdout, rpiZshCompletion)
		return 0
	default:
		_, _ = fmt.Fprintf(stdout, "unsupported shell: %s\n", shell)
		return 2
	}
}

const rpiBashCompletion = `_kbd_rpi()
{
	local cur prev config_args targets commands first_command skip_next
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[COMP_CWORD-1]}"
	commands="switch status list disconnect completion"

	if [[ "$prev" == "--config" || "$prev" == "--state" ]]; then
		COMPREPLY=( $(compgen -f -- "$cur") )
		return 0
	fi

	config_args=()
	first_command=""
	skip_next=0
	for ((i=1; i<COMP_CWORD; i++)); do
		if [[ $skip_next -eq 1 ]]; then
			skip_next=0
			continue
		fi
		if [[ "${COMP_WORDS[i]}" == "--config" ]]; then
			if [[ $((i + 1)) -lt $COMP_CWORD ]]; then
				config_args=(--config "${COMP_WORDS[i + 1]}")
			fi
			skip_next=1
			continue
		fi
		if [[ "${COMP_WORDS[i]}" == "--state" ]]; then
			skip_next=1
			continue
		fi
		if [[ "${COMP_WORDS[i]}" == -* ]]; then
			continue
		fi
		if [[ -z "$first_command" ]]; then
			first_command="${COMP_WORDS[i]}"
		fi
	done

	if [[ "$prev" == "switch" ]]; then
		targets="$(kbd-rpi "${config_args[@]}" __complete-targets 2>/dev/null)"
		COMPREPLY=( $(compgen -W "$targets" -- "$cur") )
		return 0
	fi

	if [[ "$prev" == "completion" ]]; then
		COMPREPLY=( $(compgen -W "bash zsh" -- "$cur") )
		return 0
	fi

	if [[ -z "$first_command" ]]; then
		COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
		return 0
	fi
}

complete -F _kbd_rpi kbd-rpi
`

const rpiZshCompletion = `#compdef kbd-rpi

_kbd_rpi()
{
	local -a commands targets config_args
	local first_command skip_next
	commands=(switch status list disconnect completion)

	if [[ ${words[CURRENT-1]} == "--config" || ${words[CURRENT-1]} == "--state" ]]; then
		_files
		return
	fi

	config_args=()
	first_command=""
	skip_next=0
	for ((i=2; i<CURRENT; i++)); do
		if [[ $skip_next -eq 1 ]]; then
			skip_next=0
			continue
		fi
		if [[ ${words[i]} == "--config" ]]; then
			if [[ $((i + 1)) -lt $CURRENT ]]; then
				config_args=(--config ${words[i + 1]})
			fi
			skip_next=1
			continue
		fi
		if [[ ${words[i]} == "--state" ]]; then
			skip_next=1
			continue
		fi
		if [[ ${words[i]} == -* ]]; then
			continue
		fi
		if [[ -z $first_command ]]; then
			first_command=${words[i]}
		fi
	done

	if [[ ${words[CURRENT-1]} == "switch" ]]; then
		targets=("${(@f)$(kbd-rpi "${config_args[@]}" __complete-targets 2>/dev/null)}")
		_describe 'target' targets
		return
	fi

	if [[ ${words[CURRENT-1]} == "completion" ]]; then
		_values 'shell' bash zsh
		return
	fi

	if [[ -z $first_command ]]; then
		_describe 'command' commands
		return
	fi
}

_kbd_rpi "$@"
`

func (app App) runBluetoothctl(args ...string) error {
	if err := app.runner().Run(app.context(), app.stdin(), app.stdout(), app.stderr(), "bluetoothctl", args...); err != nil {
		return fmt.Errorf("run bluetoothctl: %w", err)
	}

	return nil
}

func (app App) runner() execx.Runner {
	if app.Runner == nil {
		return execx.OSRunner{}
	}

	return app.Runner
}

func (app App) context() context.Context {
	if app.Context == nil {
		return context.Background()
	}

	return app.Context
}

func (app App) stdin() io.Reader {
	if app.Stdin == nil {
		return os.Stdin
	}

	return app.Stdin
}

func (app App) stdout() io.Writer {
	if app.Stdout == nil {
		return os.Stdout
	}

	return app.Stdout
}

func (app App) stderr() io.Writer {
	if app.Stderr == nil {
		return os.Stderr
	}

	return app.Stderr
}

func (app App) now() time.Time {
	if app.Now == nil {
		return time.Now()
	}

	return app.Now()
}

func (app App) sleep(duration time.Duration) {
	if duration <= 0 {
		return
	}
	if app.Sleep == nil {
		time.Sleep(duration)
		return
	}

	app.Sleep(duration)
}
