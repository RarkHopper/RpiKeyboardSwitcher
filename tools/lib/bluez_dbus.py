from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import TYPE_CHECKING, cast

import dbus

if TYPE_CHECKING:
    from gi.repository import GLib

BLUEZ = "org.bluez"

type DBusValue = str | bool | int | float | bytes | None | list[DBusValue] | Mapping[str, DBusValue]
type Properties = Mapping[str, DBusValue]
type Interfaces = Mapping[str, Properties]
type ManagedObjects = Mapping[str, Interfaces]
type DBusCallable = Callable[..., DBusValue]


class DBusConnection(ABC):
    @property
    @abstractmethod
    def raw(self) -> dbus.SystemBus: ...

    @abstractmethod
    def get_object(self, path: str) -> DBusRemoteObject: ...


@dataclass(frozen=True)
class SystemBusConnection(DBusConnection):
    _raw: dbus.SystemBus

    @property
    def raw(self) -> dbus.SystemBus:
        return self._raw

    def get_object(self, path: str) -> DBusRemoteObject:
        return DBusRemoteObject(self.raw.get_object(BLUEZ, path))


@dataclass(frozen=True)
class DBusRemoteObject:
    raw: dbus.RemoteObject


class DBusProxy(ABC):
    @abstractmethod
    def call(self, method_name: str, *args: DBusValue) -> DBusValue: ...

    @abstractmethod
    def call_with_timeout(self, method_name: str, timeout: float) -> None: ...


@dataclass(frozen=True)
class DBusInterface(DBusProxy):
    raw: dbus.Interface

    def call(self, method_name: str, *args: DBusValue) -> DBusValue:
        candidate = getattr(self.raw, method_name, None)
        if not callable(candidate):
            raise TypeError(f"DBus proxy does not expose callable {method_name}")
        method = cast(DBusCallable, candidate)
        return method(*args)

    def call_with_timeout(self, method_name: str, timeout: float) -> None:
        candidate = getattr(self.raw, method_name, None)
        if not callable(candidate):
            raise TypeError(f"DBus proxy does not expose callable {method_name}")
        method = cast(DBusCallable, candidate)
        method(timeout=timeout)


class GMainLoop(ABC):
    @abstractmethod
    def run(self) -> None: ...

    @abstractmethod
    def quit(self) -> None: ...


@dataclass(frozen=True)
class GMainLoopProxy(GMainLoop):
    raw: GLib.MainLoop

    def run(self) -> None:
        self.raw.run()

    def quit(self) -> None:
        self.raw.quit()


def system_bus() -> DBusConnection:
    return SystemBusConnection(dbus.SystemBus())


def bluez_object(bus: DBusConnection, path: str) -> DBusRemoteObject:
    return bus.get_object(path)


def dbus_interface(obj: DBusRemoteObject, interface: str) -> DBusProxy:
    return DBusInterface(dbus.Interface(obj.raw, interface))


def call_dbus(proxy: DBusProxy, method_name: str, *args: DBusValue) -> DBusValue:
    return proxy.call(method_name, *args)


def call_dbus_with_timeout(proxy: DBusProxy, method_name: str, timeout: float) -> None:
    proxy.call_with_timeout(method_name, timeout)


def call_loop(loop: GMainLoop, method_name: str) -> None:
    if method_name == "run":
        loop.run()
        return
    if method_name == "quit":
        loop.quit()
        return
    raise TypeError(f"GLib main loop does not expose callable {method_name}")


def dbus_true() -> DBusValue:
    return cast(DBusValue, dbus.Boolean(True))
