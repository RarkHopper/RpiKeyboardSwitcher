package hidapp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/bluez"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/input"
)

type HIDDaemon interface {
	Run(ctx context.Context, options bluez.DaemonOptions) error
}

type InputForwarder interface {
	Descriptor() (input.Descriptor, error)
	Run(ctx context.Context, send func(input.Report) error) error
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
			_, _ = fmt.Fprintln(app.stderr(), "usage: kbd-hid [--config path] daemon")
			return 2
		}

		return app.daemon(path)
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

func (app App) daemon(configPath string) int {
	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 2
	}
	forwarder := app.inputForwarder(cfg)
	descriptor, err := forwarder.Descriptor()
	if err != nil {
		_, _ = fmt.Fprintln(app.stderr(), err)
		return 1
	}

	options := app.daemonOptions(configPath, cfg, descriptor, forwarder)
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
	_, _ = fmt.Fprintf(app.stdout(), "hidraw_device: %s\n", cfg.HID.HIDRawDevice)
	if descriptor, err := app.inputForwarder(cfg).Descriptor(); err == nil {
		_, _ = fmt.Fprintf(app.stdout(), "report_map_bytes: %d\n", len(descriptor.ReportMap))
		_, _ = fmt.Fprintf(app.stdout(), "input_report_ids: %s\n", reportIDsString(descriptor.InputReportIDs))
	} else {
		_, _ = fmt.Fprintf(app.stdout(), "report_map_error: %v\n", err)
	}
	_, _ = fmt.Fprintf(app.stdout(), "gatt_root: %s\n", bluez.AppPath)
	_, _ = fmt.Fprintf(app.stdout(), "advertisement: %s\n", bluez.AdvertisementPath)
	_, _ = fmt.Fprintf(app.stdout(), "service_uuid: %s\n", bluez.HIDServiceUUID)

	return 0
}

func (app App) daemonOptions(configPath string, cfg config.RPIConfig, descriptor input.Descriptor, forwarder InputForwarder) bluez.DaemonOptions {
	return bluez.DaemonOptions{
		Adapter:         cfg.HID.Adapter,
		Name:            cfg.HID.Name,
		Appearance:      bluez.KeyboardAppearance,
		Pairable:        cfg.HID.PairableEnabled(),
		Discoverable:    cfg.HID.DiscoverableEnabled(),
		ReportMap:       descriptor.ReportMap,
		InputReportIDs:  descriptor.InputReportIDs,
		OutputReportIDs: descriptor.OutputReportIDs,
		InputReports: func(ctx context.Context, send func(bluez.InputReport) error) error {
			return forwarder.Run(ctx, func(report input.Report) error {
				return send(bluez.InputReport{
					ID:   report.ID,
					Data: report.Data,
				})
			})
		},
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

func (app App) inputForwarder(cfg config.RPIConfig) InputForwarder {
	if app.Input != nil {
		return app.Input
	}

	return input.Forwarder{
		Device: cfg.HID.HIDRawDevice,
		Log:    app.stderr(),
	}
}

func reportIDsString(ids []byte) string {
	if len(ids) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("0x%02X", id))
	}

	return strings.Join(parts, ", ")
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
