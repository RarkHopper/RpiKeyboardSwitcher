#!/usr/bin/env python3
from __future__ import annotations

import argparse
import contextlib
import sys
import time
from dataclasses import dataclass
from typing import cast

import dbus

from lib.bluez_dbus import (
    DBusConnection,
    DBusProxy,
    DBusValue,
    ManagedObjects,
    Properties,
    bluez_object,
    call_dbus,
    call_dbus_with_timeout,
    dbus_interface,
    dbus_true,
    system_bus,
)

OBJECT_MANAGER = "org.freedesktop.DBus.ObjectManager"
PROPERTIES = "org.freedesktop.DBus.Properties"
ADAPTER = "org.bluez.Adapter1"
DEVICE = "org.bluez.Device1"


@dataclass(frozen=True)
class DeviceSnapshot:
    path: str
    props: Properties


class BlueZClient:
    def __init__(self, bus: DBusConnection) -> None:
        self._bus = bus

    @classmethod
    def from_system_bus(cls) -> BlueZClient:
        return cls(system_bus())

    def _interface(self, path: str, interface: str) -> DBusProxy:
        return dbus_interface(bluez_object(self._bus, path), interface)

    def managed_objects(self) -> ManagedObjects:
        manager = self._interface("/", OBJECT_MANAGER)
        return cast(ManagedObjects, call_dbus(manager, "GetManagedObjects"))

    def adapter(self, name: str) -> AdapterProxy:
        return AdapterProxy(self._interface(adapter_path(name), ADAPTER))

    def device(self, path: str) -> DeviceProxy:
        return DeviceProxy(self._interface(path, DEVICE))

    def get_props(self, path: str, interface: str) -> Properties:
        props = self._interface(path, PROPERTIES)
        return cast(Properties, call_dbus(props, "GetAll", interface))

    def set_prop(self, path: str, interface: str, name: str, value: DBusValue) -> None:
        props = self._interface(path, PROPERTIES)
        call_dbus(props, "Set", interface, name, value)


class AdapterProxy:
    def __init__(self, proxy: DBusProxy) -> None:
        self._proxy = proxy

    def remove_device(self, path: str) -> None:
        call_dbus(self._proxy, "RemoveDevice", path)

    def start_discovery(self) -> None:
        call_dbus(self._proxy, "StartDiscovery")

    def stop_discovery(self) -> None:
        call_dbus(self._proxy, "StopDiscovery")


class DeviceProxy:
    def __init__(self, proxy: DBusProxy) -> None:
        self._proxy = proxy

    def pair(self, timeout: float) -> None:
        call_dbus_with_timeout(self._proxy, "Pair", timeout)

    def connect(self, timeout: float) -> None:
        call_dbus_with_timeout(self._proxy, "Connect", timeout)


def adapter_path(adapter: str) -> str:
    return f"/org/bluez/{adapter}"


def find_device(client: BlueZClient, address: str) -> DeviceSnapshot | None:
    want = address.upper()
    for path, interfaces in client.managed_objects().items():
        props = interfaces.get(DEVICE)
        if props and str(props.get("Address", "")).upper() == want:
            return DeviceSnapshot(path, props)
    return None


def wait_for_device(client: BlueZClient, address: str, timeout: float) -> DeviceSnapshot:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        device = find_device(client, address)
        if device is not None:
            return device
        time.sleep(0.2)
    raise TimeoutError(f"device {address} was not discovered")


def bool_text(value: DBusValue) -> str:
    return "yes" if bool(value) else "no"


def prop_text(value: DBusValue) -> str:
    if isinstance(value, bytes):
        return value.decode()
    return str(value)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("address")
    parser.add_argument("--adapter", default="hci0")
    parser.add_argument("--discover-timeout", type=float, default=45)
    parser.add_argument("--connect-timeout", type=float, default=45)
    args = parser.parse_args()

    address = cast(str, args.address)
    adapter_name = cast(str, args.adapter)
    discover_timeout = cast(float, args.discover_timeout)
    connect_timeout = cast(float, args.connect_timeout)

    client = BlueZClient.from_system_bus()
    adapter = adapter_path(adapter_name)
    adapter_proxy = client.adapter(adapter_name)

    existing_device = find_device(client, address)
    if existing_device is not None:
        with contextlib.suppress(dbus.DBusException):
            adapter_proxy.remove_device(existing_device.path)

    client.set_prop(adapter, ADAPTER, "Powered", dbus_true())

    adapter_proxy.start_discovery()
    try:
        device = wait_for_device(client, address, discover_timeout)
    finally:
        with contextlib.suppress(dbus.DBusException):
            adapter_proxy.stop_discovery()

    device_proxy = client.device(device.path)

    props = client.get_props(device.path, DEVICE)
    if not bool(props.get("Paired", False)):
        device_proxy.pair(timeout=connect_timeout)

    client.set_prop(device.path, DEVICE, "Trusted", dbus_true())

    props = client.get_props(device.path, DEVICE)
    if not bool(props.get("Connected", False)):
        device_proxy.connect(timeout=connect_timeout)

    deadline = time.monotonic() + connect_timeout
    while time.monotonic() < deadline:
        props = client.get_props(device.path, DEVICE)
        if bool(props.get("Paired", False)) and bool(props.get("Connected", False)):
            break
        time.sleep(0.2)

    props = client.get_props(device.path, DEVICE)
    print(f"Device: {address.upper()}", flush=True)
    print(f"Name: {prop_text(props.get('Name', ''))}", flush=True)
    print(f"Paired: {bool_text(props.get('Paired', False))}", flush=True)
    print(f"Bonded: {bool_text(props.get('Bonded', False))}", flush=True)
    print(f"Trusted: {bool_text(props.get('Trusted', False))}", flush=True)
    print(f"Connected: {bool_text(props.get('Connected', False))}", flush=True)

    if not bool(props.get("Paired", False)):
        raise RuntimeError("device is not paired")
    if not bool(props.get("Trusted", False)):
        raise RuntimeError("device is not trusted")
    if not bool(props.get("Connected", False)):
        raise RuntimeError("device is not connected")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"bluez-pair: {error}", file=sys.stderr, flush=True)
        sys.exit(1)
