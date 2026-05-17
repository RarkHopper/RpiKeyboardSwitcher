package bluez

import (
	"fmt"
	"io"

	"github.com/godbus/dbus/v5"
)

type Agent struct {
	log io.Writer
}

func NewAgent(log io.Writer) *Agent {
	return &Agent{log: log}
}

func (agent *Agent) Export(conn *dbus.Conn) error {
	if err := conn.Export(agent, AgentPath, AgentInterface); err != nil {
		return fmt.Errorf("export pairing agent: %w", err)
	}

	return nil
}

func (agent *Agent) Release() *dbus.Error {
	agent.logf("BlueZ released the pairing agent\n")
	return nil
}

func (agent *Agent) RequestPinCode(device dbus.ObjectPath) (string, *dbus.Error) {
	agent.logf("Pairing PIN requested from %s; returning 000000\n", device)
	return "000000", nil
}

func (agent *Agent) DisplayPinCode(device dbus.ObjectPath, pin string) *dbus.Error {
	agent.logf("Pairing PIN for %s: %s\n", device, pin)
	return nil
}

func (agent *Agent) RequestPasskey(device dbus.ObjectPath) (uint32, *dbus.Error) {
	agent.logf("Pairing passkey requested from %s; returning 000000\n", device)
	return 0, nil
}

func (agent *Agent) DisplayPasskey(device dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	agent.logf("Pairing passkey for %s: %06d (%d digits entered)\n", device, passkey, entered)
	return nil
}

func (agent *Agent) RequestConfirmation(device dbus.ObjectPath, passkey uint32) *dbus.Error {
	agent.logf("Pairing confirmation for %s: %06d\n", device, passkey)
	return nil
}

func (agent *Agent) RequestAuthorization(device dbus.ObjectPath) *dbus.Error {
	agent.logf("Pairing authorization requested from %s\n", device)
	return nil
}

func (agent *Agent) AuthorizeService(device dbus.ObjectPath, uuid string) *dbus.Error {
	agent.logf("Service authorization requested from %s for %s\n", device, uuid)
	return nil
}

func (agent *Agent) Cancel() *dbus.Error {
	agent.logf("Pairing request canceled\n")
	return nil
}

func (agent *Agent) logf(format string, args ...any) {
	if agent.log == nil {
		return
	}

	_, _ = fmt.Fprintf(agent.log, format, args...)
}
