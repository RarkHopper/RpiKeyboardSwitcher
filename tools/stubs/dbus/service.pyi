from __future__ import annotations

from collections.abc import Callable
from typing import ParamSpec, TypeVar

from dbus import SystemBus

_P = ParamSpec("_P")
_R = TypeVar("_R")

class Object:
    def __init__(self, bus: SystemBus, object_path: str) -> None: ...

def method(
    dbus_interface: str,
    *,
    in_signature: str,
    out_signature: str,
) -> Callable[[Callable[_P, _R]], Callable[_P, _R]]: ...
