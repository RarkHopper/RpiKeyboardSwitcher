from __future__ import annotations

from collections.abc import Mapping
from typing import NewType, cast

import dbus

BLUEZ = "org.bluez"

type DBusValue = str | bool | int | float | bytes | None | list[DBusValue] | Mapping[str, DBusValue]
type Properties = Mapping[str, DBusValue]
type Interfaces = Mapping[str, Properties]
type ManagedObjects = Mapping[str, Interfaces]

DBusConnection = NewType("DBusConnection", object)
DBusRemoteObject = NewType("DBusRemoteObject", object)
DBusProxy = NewType("DBusProxy", object)
GMainLoop = NewType("GMainLoop", object)


def system_bus() -> DBusConnection:
    return DBusConnection(dbus.SystemBus())


def bluez_object(bus: DBusConnection, path: str) -> DBusRemoteObject:
    get_object = getattr(bus, "get_object", None)
    if not callable(get_object):
        raise TypeError("DBus connection does not expose callable get_object")
    return DBusRemoteObject(get_object(BLUEZ, path))


def dbus_interface(obj: DBusRemoteObject, interface: str) -> DBusProxy:
    return DBusProxy(dbus.Interface(obj, interface))


def call_dbus(proxy: DBusProxy, method_name: str, *args: DBusValue) -> DBusValue:
    method = getattr(proxy, method_name, None)
    if not callable(method):
        raise TypeError(f"DBus proxy does not expose callable {method_name}")
    return cast(DBusValue, method(*args))


def call_dbus_with_timeout(proxy: DBusProxy, method_name: str, timeout: float) -> None:
    method = getattr(proxy, method_name, None)
    if not callable(method):
        raise TypeError(f"DBus proxy does not expose callable {method_name}")
    method(timeout=timeout)


def call_loop(loop: GMainLoop, method_name: str) -> None:
    method = getattr(loop, method_name, None)
    if not callable(method):
        raise TypeError(f"GLib main loop does not expose callable {method_name}")
    method()


def dbus_true() -> DBusValue:
    return cast(DBusValue, dbus.Boolean(True))
