#!/usr/bin/env python3
from __future__ import annotations

import argparse
import signal
import sys
from types import FrameType
from typing import cast

import dbus
import dbus.mainloop.glib
import dbus.service
from gi.repository import GLib

from lib.bluez_dbus import (
    DBusConnection,
    GMainLoop,
    bluez_object,
    call_dbus,
    call_loop,
    dbus_interface,
    system_bus,
)

BLUEZ = "org.bluez"
AGENT_MANAGER = "org.bluez.AgentManager1"
AGENT = "org.bluez.Agent1"
AGENT_PATH = "/com/rarkhopper/RpiKeyboardSwitcher/testagent"


class AgentManager:
    def __init__(self, bus: DBusConnection) -> None:
        self._proxy = dbus_interface(bluez_object(bus, "/org/bluez"), AGENT_MANAGER)

    def register_agent(self, path: str, capability: str) -> None:
        call_dbus(self._proxy, "RegisterAgent", path, capability)

    def request_default_agent(self, path: str) -> None:
        call_dbus(self._proxy, "RequestDefaultAgent", path)

    def unregister_agent(self, path: str) -> None:
        call_dbus(self._proxy, "UnregisterAgent", path)


class Agent(dbus.service.Object):
    @dbus.service.method(AGENT, in_signature="", out_signature="")
    def Release(self) -> None:
        print("agent released", flush=True)
        call_loop(loop, "quit")

    @dbus.service.method(AGENT, in_signature="o", out_signature="s")
    def RequestPinCode(self, device: str) -> str:
        print(f"request pin code device={device}", flush=True)
        return "000000"

    @dbus.service.method(AGENT, in_signature="os", out_signature="")
    def DisplayPinCode(self, device: str, pincode: str) -> None:
        print(f"display pin code device={device} pincode={pincode}", flush=True)

    @dbus.service.method(AGENT, in_signature="ouq", out_signature="")
    def DisplayPasskey(self, device: str, passkey: int, entered: int) -> None:
        print(
            f"display passkey device={device} passkey={passkey:06d} entered={entered}", flush=True
        )

    @dbus.service.method(AGENT, in_signature="o", out_signature="u")
    def RequestPasskey(self, device: str) -> int:
        print(f"request passkey device={device}", flush=True)
        return cast(int, dbus.UInt32(0))

    @dbus.service.method(AGENT, in_signature="ou", out_signature="")
    def RequestConfirmation(self, device: str, passkey: int) -> None:
        print(f"confirm device={device} passkey={passkey:06d}", flush=True)

    @dbus.service.method(AGENT, in_signature="o", out_signature="")
    def RequestAuthorization(self, device: str) -> None:
        print(f"authorize pairing device={device}", flush=True)

    @dbus.service.method(AGENT, in_signature="os", out_signature="")
    def AuthorizeService(self, device: str, uuid: str) -> None:
        print(f"authorize service device={device} uuid={uuid}", flush=True)

    @dbus.service.method(AGENT, in_signature="", out_signature="")
    def Cancel(self) -> None:
        print("request canceled", flush=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--capability", default="KeyboardDisplay")
    args = parser.parse_args()
    capability = cast(str, args.capability)

    dbus.mainloop.glib.DBusGMainLoop(set_as_default=True)
    bus = system_bus()
    Agent(bus, AGENT_PATH)

    manager = AgentManager(bus)
    manager.register_agent(AGENT_PATH, capability)
    manager.request_default_agent(AGENT_PATH)
    print(f"agent registered path={AGENT_PATH} capability={capability}", flush=True)

    def stop(_signum: int, _frame: FrameType | None) -> None:
        manager.unregister_agent(AGENT_PATH)
        call_loop(loop, "quit")

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    call_loop(loop, "run")


loop = GMainLoop(GLib.MainLoop())


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"bluez-agent: {error}", file=sys.stderr, flush=True)
        sys.exit(1)
