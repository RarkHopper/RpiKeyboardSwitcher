package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errIncompleteState = errors.New("state is incomplete")

type State struct {
	Target       string    `json:"target"`
	BluetoothMAC string    `json:"bluetooth_mac"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func Load(path string) (State, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, fmt.Errorf("open state: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var current State
	if err := json.NewDecoder(file).Decode(&current); err != nil {
		return State{}, true, fmt.Errorf("decode state: %w", err)
	}
	if current.Target == "" || current.BluetoothMAC == "" || current.UpdatedAt.IsZero() {
		return State{}, true, errIncompleteState
	}

	return current, true, nil
}

func Save(path string, current State) (err error) {
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o750); mkdirErr != nil {
		return fmt.Errorf("create state directory: %w", mkdirErr)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create state: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close state: %w", closeErr)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(current); err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	return nil
}

func Remove(path string) error {
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("remove state: %w", err)
	}

	return nil
}
