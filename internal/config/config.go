package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

const (
	DefaultRPIConfigPath = "/etc/kbd-switch/config.yaml"
	DefaultStatePath     = "/run/kbd-switch/state.json"

	DefaultHIDAdapter     = "hci0"
	DefaultHIDName        = "Rpi Keyboard Switcher"
	HIDAppearanceKeyboard = "keyboard"
)

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	macPattern  = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)
)

type LocalConfig struct {
	RPI LocalRPIConfig `yaml:"rpi"`
}

type LocalRPIConfig struct {
	Host          string `yaml:"host"`
	User          string `yaml:"user"`
	RemoteCommand string `yaml:"remote_command"`
}

type RPIConfig struct {
	Targets  map[string]Target `yaml:"targets,omitempty"`
	Behavior Behavior          `yaml:"behavior"`
	HID      HIDConfig         `yaml:"hid"`
}

type Target struct {
	Name         string `yaml:"name"`
	BluetoothMAC string `yaml:"bluetooth_mac"`
}

type Behavior struct {
	DisconnectOthers *bool `yaml:"disconnect_others,omitempty"`
	ReconnectWaitSec int   `yaml:"reconnect_wait_sec,omitempty"`
}

type HIDConfig struct {
	Adapter      string `yaml:"adapter"`
	Name         string `yaml:"name"`
	Appearance   string `yaml:"appearance"`
	Pairable     *bool  `yaml:"pairable,omitempty"`
	Discoverable *bool  `yaml:"discoverable,omitempty"`
	HIDRawDevice string `yaml:"hidraw_device"`
}

func DefaultLocalConfigPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}

	return filepath.Join(configHome, "kbd-switch", "config.yaml"), nil
}

func LoadLocal(path string) (LocalConfig, error) {
	var cfg LocalConfig
	if err := loadYAML(path, &cfg); err != nil {
		return LocalConfig{}, err
	}
	if err := cfg.Validate(); err != nil {
		return LocalConfig{}, err
	}

	return cfg, nil
}

func LoadRPI(path string) (RPIConfig, error) {
	var cfg RPIConfig
	if err := loadYAML(path, &cfg); err != nil {
		return RPIConfig{}, err
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return RPIConfig{}, err
	}

	return cfg, nil
}

func SaveRPI(path string, cfg RPIConfig) error {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Targets == nil {
		cfg.Targets = map[string]Target{}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		_ = file.Close()
		return fmt.Errorf("close config encoder: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("chmod temporary config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}

	return nil
}

func (cfg LocalConfig) Validate() error {
	if strings.TrimSpace(cfg.RPI.Host) == "" {
		return errors.New("rpi.host is required")
	}
	if hasSpace(cfg.RPI.Host) {
		return errors.New("rpi.host must not contain whitespace")
	}
	if strings.TrimSpace(cfg.RPI.User) == "" {
		return errors.New("rpi.user is required")
	}
	if hasSpace(cfg.RPI.User) {
		return errors.New("rpi.user must not contain whitespace")
	}
	if strings.TrimSpace(cfg.RPI.RemoteCommand) == "" {
		return errors.New("rpi.remote_command is required")
	}
	if hasSpace(cfg.RPI.RemoteCommand) {
		return errors.New("rpi.remote_command must not contain whitespace")
	}
	return nil
}

func (cfg RPIConfig) Validate() error {
	if cfg.Behavior.ReconnectWaitSec < 0 {
		return errors.New("behavior.reconnect_wait_sec must not be negative")
	}
	if err := cfg.HID.Validate(); err != nil {
		return err
	}
	for key, target := range cfg.Targets {
		if err := validateName("targets key", key); err != nil {
			return err
		}
		if err := ValidateTarget("targets."+key, target); err != nil {
			return err
		}
	}

	return nil
}

func ValidateTarget(field string, target Target) error {
	if strings.TrimSpace(target.Name) == "" {
		return fmt.Errorf("%s.name is required", field)
	}
	if !macPattern.MatchString(target.BluetoothMAC) {
		return fmt.Errorf("%s.bluetooth_mac must be uppercase Bluetooth MAC address", field)
	}

	return nil
}

func (cfg *RPIConfig) ApplyDefaults() {
	if cfg.HID.Adapter == "" {
		cfg.HID.Adapter = DefaultHIDAdapter
	}
	if cfg.HID.Name == "" {
		cfg.HID.Name = DefaultHIDName
	}
	if cfg.HID.Appearance == "" {
		cfg.HID.Appearance = HIDAppearanceKeyboard
	}
}

func (hid HIDConfig) Validate() error {
	if err := validateName("hid.adapter", hid.Adapter); err != nil {
		return err
	}
	if strings.TrimSpace(hid.Name) == "" {
		return errors.New("hid.name is required")
	}
	if hasControl(hid.Name) {
		return errors.New("hid.name must not contain control characters")
	}
	if hid.Appearance != HIDAppearanceKeyboard {
		return errors.New("hid.appearance must be keyboard")
	}
	if strings.TrimSpace(hid.HIDRawDevice) == "" {
		return errors.New("hid.hidraw_device is required")
	}
	if hasControl(hid.HIDRawDevice) {
		return errors.New("hid.hidraw_device must not contain control characters")
	}

	return nil
}

func (behavior Behavior) ShouldDisconnectOthers() bool {
	return behavior.DisconnectOthers == nil || *behavior.DisconnectOthers
}

func (hid HIDConfig) PairableEnabled() bool {
	return hid.Pairable == nil || *hid.Pairable
}

func (hid HIDConfig) DiscoverableEnabled() bool {
	return hid.Discoverable == nil || *hid.Discoverable
}

func loadYAML(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	return nil
}

func validateName(field string, value string) error {
	if !namePattern.MatchString(value) {
		return fmt.Errorf("%s must contain only letters, digits, '_', '-', '.'", field)
	}

	return nil
}

func ValidateName(field string, value string) error {
	return validateName(field, value)
}

func hasSpace(value string) bool {
	for _, char := range value {
		if unicode.IsSpace(char) {
			return true
		}
	}

	return false
}

func hasControl(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}

	return false
}
