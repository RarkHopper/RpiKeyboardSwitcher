package hidapp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/bluez"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidreport"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/input"
)

type HIDDaemon interface {
	Run(ctx context.Context, options bluez.DaemonOptions) error
}

type InputForwarder interface {
	Run(ctx context.Context, send func([]byte) error) error
}

type App struct {
	Daemon  HIDDaemon
	Input   InputForwarder
	Context context.Context
	Stdout  io.Writer
	Stderr  io.Writer
}

func (app App) Run(args []string) int {
	options, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	if options.command == "" {
		_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-hid [--config path] <daemon|inspect>")
		return 2
	}

	path := resolveConfigPath(options.configPath)
	switch options.command {
	case "daemon":
		if len(options.operands) != 0 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-hid [--config path] daemon [--test-text text]")
			return 2
		}

		return app.daemon(path, options.testText)
	case "inspect":
		if len(options.operands) != 0 {
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-hid [--config path] inspect")
			return 2
		}

		return app.inspect(path)
	default:
		_, _ = fmt.Fprintf(app.stderr(), "unknown command: %s\n", options.command)
		return 2
	}
}

type cliOptions struct {
	configPath string
	testText   string
	command    string
	operands   []string
}

func parseArgs(args []string) (cliOptions, error) {
	var options cliOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--config":
			value, next, err := requireFlagValue(args, index, "--config")
			if err != nil {
				return cliOptions{}, err
			}
			options.configPath = value
			index = next
		case strings.HasPrefix(arg, "--config="):
			options.configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--test-text":
			value, next, err := requireFlagValue(args, index, "--test-text")
			if err != nil {
				return cliOptions{}, err
			}
			options.testText = value
			index = next
		case strings.HasPrefix(arg, "--test-text="):
			options.testText = strings.TrimPrefix(arg, "--test-text=")
		case strings.HasPrefix(arg, "-"):
			return cliOptions{}, fmt.Errorf("unknown flag: %s", arg)
		case options.command == "":
			options.command = arg
		default:
			options.operands = append(options.operands, arg)
		}
	}

	return options, nil
}

func requireFlagValue(args []string, index int, name string) (string, int, error) {
	next := index + 1
	if next >= len(args) || strings.HasPrefix(args[next], "-") {
		return "", index, fmt.Errorf("%s requires a value", name)
	}

	return args[next], next, nil
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

func (app App) daemon(configPath string, testText string) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}
	reports, err := testReports(testText)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	options := app.daemonOptions(configPath, cfg, reports)
	if err := app.daemonRunner().Run(app.context(), options); err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}

	return 0
}

func (app App) inspect(configPath string) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}

	_, _ = fmt.Fprintf(app.stdout(), "adapter: %s\n", cfg.HID.Adapter)
	_, _ = fmt.Fprintf(app.stdout(), "name: %s\n", cfg.HID.Name)
	_, _ = fmt.Fprintf(app.stdout(), "appearance: %s (0x%04X)\n", cfg.HID.Appearance, bluez.KeyboardAppearance)
	_, _ = fmt.Fprintf(app.stdout(), "pairable: %t\n", cfg.HID.PairableEnabled())
	_, _ = fmt.Fprintf(app.stdout(), "discoverable: %t\n", cfg.HID.DiscoverableEnabled())
	if len(cfg.HID.InputDevices) == 0 {
		_, _ = fmt.Fprintf(app.stdout(), "input_devices: %s (default)\n", input.DefaultKeyboardGlob)
	} else {
		_, _ = fmt.Fprintf(app.stdout(), "input_devices: %s\n", strings.Join(cfg.HID.InputDevices, ", "))
	}
	_, _ = fmt.Fprintf(app.stdout(), "gatt_root: %s\n", bluez.AppPath)
	_, _ = fmt.Fprintf(app.stdout(), "advertisement: %s\n", bluez.AdvertisementPath)
	_, _ = fmt.Fprintf(app.stdout(), "service_uuid: %s\n", bluez.HIDServiceUUID)

	return 0
}

func (app App) daemonOptions(configPath string, cfg config.RPIConfig, reports [][]byte) bluez.DaemonOptions {
	return bluez.DaemonOptions{
		Adapter:      cfg.HID.Adapter,
		Name:         cfg.HID.Name,
		Appearance:   bluez.KeyboardAppearance,
		Pairable:     cfg.HID.PairableEnabled(),
		Discoverable: cfg.HID.DiscoverableEnabled(),
		TestReports:  reports,
		InputReports: app.inputForwarder(cfg).Run,
		OnPeerReady: func(peer bluez.Peer) error {
			return app.cachePeer(configPath, peer)
		},
		Log: app.stderr(),
	}
}

func (app App) cachePeer(configPath string, peer bluez.Peer) error {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		return err
	}
	if cfg.Targets == nil {
		cfg.Targets = map[string]config.Target{}
	}
	for _, target := range cfg.Targets {
		if target.BluetoothMAC == peer.BluetoothMAC {
			return nil
		}
	}

	key := uniqueTargetKey(cfg.Targets, peer.Name)
	cfg.Targets[key] = config.Target{
		Name:         peer.Name,
		BluetoothMAC: peer.BluetoothMAC,
	}

	return config.SaveRPI(configPath, cfg)
}

func uniqueTargetKey(targets map[string]config.Target, name string) string {
	base := targetKey(name)
	key := base
	for suffix := 2; ; suffix++ {
		if _, ok := targets[key]; !ok {
			return key
		}
		key = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func targetKey(name string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range strings.ToLower(name) {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
			lastDash = false
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		case char == '_' || char == '.':
			builder.WriteRune(char)
			lastDash = false
		default:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	key := strings.Trim(builder.String(), "-")
	if key == "" {
		return "target"
	}

	return key
}

func testReports(text string) ([][]byte, error) {
	if text == "" {
		return nil, nil
	}

	reports, err := hidreport.ReportsForText(text)
	if err != nil {
		return nil, err
	}

	return hidreport.Bytes(reports), nil
}

func (app App) inputForwarder(cfg config.RPIConfig) InputForwarder {
	if app.Input != nil {
		return app.Input
	}

	return input.Forwarder{
		Paths: cfg.HID.InputDevices,
		Log:   app.stderr(),
	}
}

func (app App) daemonRunner() HIDDaemon {
	if app.Daemon == nil {
		return bluez.DBusDaemon{}
	}

	return app.Daemon
}

func (app App) context() context.Context {
	if app.Context == nil {
		return context.Background()
	}

	return app.Context
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
