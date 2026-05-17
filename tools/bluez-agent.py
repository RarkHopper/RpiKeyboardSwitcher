#!/usr/bin/env python3
import argparse
import signal
import sys

import dbus
import dbus.mainloop.glib
import dbus.service
from gi.repository import GLib


BLUEZ = "org.bluez"
AGENT_MANAGER = "org.bluez.AgentManager1"
AGENT = "org.bluez.Agent1"
AGENT_PATH = "/com/rarkhopper/RpiKeyboardSwitcher/testagent"


class Agent(dbus.service.Object):
    @dbus.service.method(AGENT, in_signature="", out_signature="")
    def Release(self):
        print("agent released", flush=True)
        loop.quit()

    @dbus.service.method(AGENT, in_signature="o", out_signature="s")
    def RequestPinCode(self, device):
        print(f"request pin code device={device}", flush=True)
        return "000000"

    @dbus.service.method(AGENT, in_signature="os", out_signature="")
    def DisplayPinCode(self, device, pincode):
        print(f"display pin code device={device} pincode={pincode}", flush=True)

    @dbus.service.method(AGENT, in_signature="ouq", out_signature="")
    def DisplayPasskey(self, device, passkey, entered):
        print(f"display passkey device={device} passkey={passkey:06d} entered={entered}", flush=True)

    @dbus.service.method(AGENT, in_signature="o", out_signature="u")
    def RequestPasskey(self, device):
        print(f"request passkey device={device}", flush=True)
        return dbus.UInt32(0)

    @dbus.service.method(AGENT, in_signature="ou", out_signature="")
    def RequestConfirmation(self, device, passkey):
        print(f"confirm device={device} passkey={passkey:06d}", flush=True)

    @dbus.service.method(AGENT, in_signature="o", out_signature="")
    def RequestAuthorization(self, device):
        print(f"authorize pairing device={device}", flush=True)

    @dbus.service.method(AGENT, in_signature="os", out_signature="")
    def AuthorizeService(self, device, uuid):
        print(f"authorize service device={device} uuid={uuid}", flush=True)

    @dbus.service.method(AGENT, in_signature="", out_signature="")
    def Cancel(self):
        print("request canceled", flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--capability", default="KeyboardDisplay")
    args = parser.parse_args()

    dbus.mainloop.glib.DBusGMainLoop(set_as_default=True)
    bus = dbus.SystemBus()
    Agent(bus, AGENT_PATH)

    manager = dbus.Interface(bus.get_object(BLUEZ, "/org/bluez"), AGENT_MANAGER)
    manager.RegisterAgent(AGENT_PATH, args.capability)
    manager.RequestDefaultAgent(AGENT_PATH)
    print(f"agent registered path={AGENT_PATH} capability={args.capability}", flush=True)

    def stop(_signum, _frame):
        manager.UnregisterAgent(AGENT_PATH)
        loop.quit()

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    loop.run()


loop = GLib.MainLoop()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"bluez-agent: {error}", file=sys.stderr, flush=True)
        sys.exit(1)
