#!/usr/bin/env python3
import argparse
import sys
import time

import dbus


BLUEZ = "org.bluez"
OBJECT_MANAGER = "org.freedesktop.DBus.ObjectManager"
PROPERTIES = "org.freedesktop.DBus.Properties"
ADAPTER = "org.bluez.Adapter1"
DEVICE = "org.bluez.Device1"


def managed_objects(bus):
    manager = dbus.Interface(bus.get_object(BLUEZ, "/"), OBJECT_MANAGER)
    return manager.GetManagedObjects()


def adapter_path(adapter):
    return f"/org/bluez/{adapter}"


def find_device(bus, address):
    want = address.upper()
    for path, interfaces in managed_objects(bus).items():
        props = interfaces.get(DEVICE)
        if props and str(props.get("Address", "")).upper() == want:
            return path, props
    return None, None


def wait_for_device(bus, address, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        path, props = find_device(bus, address)
        if path:
            return path, props
        time.sleep(0.2)
    raise TimeoutError(f"device {address} was not discovered")


def get_props(bus, path, interface):
    obj = bus.get_object(BLUEZ, path)
    return dbus.Interface(obj, PROPERTIES).GetAll(interface)


def set_prop(bus, path, interface, name, value):
    obj = bus.get_object(BLUEZ, path)
    dbus.Interface(obj, PROPERTIES).Set(interface, name, value)


def bool_text(value):
    return "yes" if bool(value) else "no"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("address")
    parser.add_argument("--adapter", default="hci0")
    parser.add_argument("--discover-timeout", type=float, default=45)
    parser.add_argument("--connect-timeout", type=float, default=45)
    args = parser.parse_args()

    bus = dbus.SystemBus()
    adapter = adapter_path(args.adapter)
    adapter_obj = bus.get_object(BLUEZ, adapter)
    adapter_iface = dbus.Interface(adapter_obj, ADAPTER)

    existing_path, _ = find_device(bus, args.address)
    if existing_path:
        try:
            adapter_iface.RemoveDevice(existing_path)
        except dbus.DBusException:
            pass

    set_prop(bus, adapter, ADAPTER, "Powered", dbus.Boolean(True))

    adapter_iface.StartDiscovery()
    try:
        device_path, _ = wait_for_device(bus, args.address, args.discover_timeout)
    finally:
        try:
            adapter_iface.StopDiscovery()
        except dbus.DBusException:
            pass

    device_obj = bus.get_object(BLUEZ, device_path)
    device_iface = dbus.Interface(device_obj, DEVICE)

    props = get_props(bus, device_path, DEVICE)
    if not bool(props.get("Paired", False)):
        device_iface.Pair(timeout=args.connect_timeout)

    set_prop(bus, device_path, DEVICE, "Trusted", dbus.Boolean(True))

    props = get_props(bus, device_path, DEVICE)
    if not bool(props.get("Connected", False)):
        device_iface.Connect(timeout=args.connect_timeout)

    deadline = time.monotonic() + args.connect_timeout
    while time.monotonic() < deadline:
        props = get_props(bus, device_path, DEVICE)
        if bool(props.get("Paired", False)) and bool(props.get("Connected", False)):
            break
        time.sleep(0.2)

    props = get_props(bus, device_path, DEVICE)
    print(f"Device: {args.address.upper()}", flush=True)
    print(f"Name: {props.get('Name', '')}", flush=True)
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
