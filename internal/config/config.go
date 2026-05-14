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
)

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	macPattern  = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)
)

type LocalConfig struct {
	RPI     LocalRPIConfig    `yaml:"rpi"`
	Targets map[string]string `yaml:"targets"`
}

type LocalRPIConfig struct {
	Host          string `yaml:"host"`
	User          string `yaml:"user"`
	RemoteCommand string `yaml:"remote_command"`
}

type RPIConfig struct {
	Devices  map[string]Device `yaml:"devices"`
	Behavior Behavior          `yaml:"behavior"`
}

type Device struct {
	Name         string `yaml:"name"`
	BluetoothMAC string `yaml:"bluetooth_mac"`
}

type Behavior struct {
	DisconnectOthers *bool `yaml:"disconnect_others"`
	ReconnectWaitSec int   `yaml:"reconnect_wait_sec"`
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
	if err := cfg.Validate(); err != nil {
		return RPIConfig{}, err
	}

	return cfg, nil
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
	if len(cfg.Targets) == 0 {
		return errors.New("targets must not be empty")
	}
	for alias, remoteTarget := range cfg.Targets {
		if err := validateName("targets key", alias); err != nil {
			return err
		}
		if err := validateName("targets value", remoteTarget); err != nil {
			return err
		}
	}

	return nil
}

func (cfg RPIConfig) Validate() error {
	if len(cfg.Devices) == 0 {
		return errors.New("devices must not be empty")
	}
	if cfg.Behavior.ReconnectWaitSec < 0 {
		return errors.New("behavior.reconnect_wait_sec must not be negative")
	}
	for key, device := range cfg.Devices {
		if err := validateName("devices key", key); err != nil {
			return err
		}
		if strings.TrimSpace(device.Name) == "" {
			return fmt.Errorf("devices.%s.name is required", key)
		}
		if !macPattern.MatchString(device.BluetoothMAC) {
			return fmt.Errorf("devices.%s.bluetooth_mac must be uppercase Bluetooth MAC address", key)
		}
	}

	return nil
}

func (behavior Behavior) ShouldDisconnectOthers() bool {
	return behavior.DisconnectOthers == nil || *behavior.DisconnectOthers
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

func hasSpace(value string) bool {
	for _, char := range value {
		if unicode.IsSpace(char) {
			return true
		}
	}

	return false
}
