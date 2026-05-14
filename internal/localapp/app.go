package localapp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/execx"
)

type App struct {
	Runner  execx.Runner
	Context context.Context
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

func (app App) Run(args []string) int {
	fs := flag.NewFlagSet("kbd", flag.ContinueOnError)
	fs.SetOutput(app.stderr())

	configPath := fs.String("config", "", "config file path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	operands := fs.Args()
	if len(operands) == 0 {
		_, _ = fmt.Fprintln(app.stderr(), "usage: kbd [--config path] <switch target|target|status|list|completion shell>")
		return 2
	}

	if operands[0] == "completion" {
		if len(operands) != 2 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd completion <bash|zsh>")
			return 2
		}

		return printLocalCompletion(app.stdout(), operands[1])
	}

	path, err := resolveConfigPath(*configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	cfg, err := config.LoadLocal(path)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	command := operands[0]
	switch command {
	case "__complete-targets":
		if len(operands) != 1 {
			return 2
		}
		return app.runSSH(cfg, "__complete-targets")
	case "switch":
		if len(operands) != 2 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd [--config path] switch <target>")
			return 2
		}
		return app.switchTarget(cfg, operands[1])
	case "list":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd [--config path] list")
			return 2
		}
		return app.runSSH(cfg, "list")
	case "status":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd [--config path] status")
			return 2
		}
		return app.runSSH(cfg, "status")
	default:
		if len(operands) != 1 {
			_, _ = fmt.Fprintf(app.stderr(), "unknown command: %s\n", command)
			return 2
		}

		return app.switchTarget(cfg, command)
	}
}

func (app App) switchTarget(cfg config.LocalConfig, target string) int {
	if err := config.ValidateName("target", target); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	return app.runSSH(cfg, "switch", target)
}

func (app App) runSSH(cfg config.LocalConfig, args ...string) int {
	sshArgs := []string{cfg.RPI.User + "@" + cfg.RPI.Host, cfg.RPI.RemoteCommand}
	sshArgs = append(sshArgs, args...)

	if err := app.runner().Run(app.context(), app.stdin(), app.stdout(), app.stderr(), "ssh", sshArgs...); err != nil {
		return execx.ExitCode(err)
	}

	return 0
}

func resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if envPath := os.Getenv("KBD_CONFIG"); envPath != "" {
		return envPath, nil
	}

	return config.DefaultLocalConfigPath()
}

func printLocalCompletion(stdout io.Writer, shell string) int {
	switch shell {
	case "bash":
		_, _ = fmt.Fprint(stdout, localBashCompletion)
		return 0
	case "zsh":
		_, _ = fmt.Fprint(stdout, localZshCompletion)
		return 0
	default:
		_, _ = fmt.Fprintf(stdout, "unsupported shell: %s\n", shell)
		return 2
	}
}

const localBashCompletion = `_kbd()
{
	local cur prev config_args targets commands first_command skip_next
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[COMP_CWORD-1]}"
	commands="switch status list completion"

	if [[ "$prev" == "--config" ]]; then
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
		if [[ "${COMP_WORDS[i]}" == -* ]]; then
			continue
		fi
		if [[ -z "$first_command" ]]; then
			first_command="${COMP_WORDS[i]}"
		fi
	done

	if [[ "$prev" == "switch" ]]; then
		targets="$(kbd "${config_args[@]}" __complete-targets 2>/dev/null)"
		COMPREPLY=( $(compgen -W "$targets" -- "$cur") )
		return 0
	fi

	if [[ "$prev" == "completion" ]]; then
		COMPREPLY=( $(compgen -W "bash zsh" -- "$cur") )
		return 0
	fi

	if [[ -z "$first_command" ]]; then
		targets="$(kbd "${config_args[@]}" __complete-targets 2>/dev/null)"
		COMPREPLY=( $(compgen -W "$commands $targets" -- "$cur") )
		return 0
	fi
}

complete -F _kbd kbd
`

const localZshCompletion = `#compdef kbd

_kbd()
{
	local -a commands targets config_args
	local first_command skip_next
	commands=(switch status list completion)

	if [[ ${words[CURRENT-1]} == "--config" ]]; then
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
		if [[ ${words[i]} == -* ]]; then
			continue
		fi
		if [[ -z $first_command ]]; then
			first_command=${words[i]}
		fi
	done

	if [[ ${words[CURRENT-1]} == "switch" ]]; then
		targets=("${(@f)$(kbd "${config_args[@]}" __complete-targets 2>/dev/null)}")
		_describe 'target' targets
		return
	fi

	if [[ ${words[CURRENT-1]} == "completion" ]]; then
		_values 'shell' bash zsh
		return
	fi

	if [[ -z $first_command ]]; then
		targets=("${(@f)$(kbd "${config_args[@]}" __complete-targets 2>/dev/null)}")
		_describe 'command' commands
		_describe 'target' targets
		return
	fi
}

_kbd "$@"
`

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
