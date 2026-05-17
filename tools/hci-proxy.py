#!/usr/bin/env python3
import argparse
import os
import selectors
import socket
import time

HCI_PRIMARY = 0x00


def h4_packet_length(buf):
    if not buf:
        return None

    packet_type = buf[0]
    if packet_type == 0xFF:
        return 2 if len(buf) >= 2 else None
    if packet_type == 0x01:
        if len(buf) < 4:
            return None
        return 4 + buf[3]
    if packet_type == 0x02:
        if len(buf) < 5:
            return None
        return 5 + buf[3] + (buf[4] << 8)
    if packet_type == 0x03:
        if len(buf) < 4:
            return None
        return 4 + buf[3]
    if packet_type == 0x04:
        if len(buf) < 3:
            return None
        return 3 + buf[2]
    if packet_type == 0x05:
        if len(buf) < 5:
            return None
        return 5 + buf[3] + ((buf[4] & 0x3F) << 8)

    raise ValueError(f"unknown H4 packet type 0x{packet_type:02x}")


def take_h4_packets(buf):
    packets = []
    while buf:
        try:
            length = h4_packet_length(buf)
        except ValueError:
            buf = buf[1:]
            continue
        if length is None or len(buf) < length:
            break
        packets.append(bytes(buf[:length]))
        buf = buf[length:]
    return packets, buf


def write_all_fd(fd, data):
    view = memoryview(data)
    while view:
        written = os.write(fd, view)
        view = view[written:]


def open_vhci():
    fd = os.open("/dev/vhci", os.O_RDWR | os.O_CLOEXEC)
    os.write(fd, bytes([0xFF, HCI_PRIMARY]))
    return fd


def connect_tcp(host, port, timeout):
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        connection = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            connection.connect((host, port))
            connection.setblocking(False)
            return connection
        except OSError as error:
            last_error = error
            connection.close()
            time.sleep(0.1)
    raise TimeoutError(f"could not connect to {host}:{port}") from last_error


def raw_proxy(left, right):
    left.setblocking(False)
    right.setblocking(False)
    selector = selectors.DefaultSelector()
    selector.register(left, selectors.EVENT_READ, right)
    selector.register(right, selectors.EVENT_READ, left)

    while True:
        for key, _ in selector.select():
            try:
                data = key.fileobj.recv(4096)
            except BlockingIOError:
                continue
            if not data:
                return
            key.data.sendall(data)


def bridge(listen_host, listen_port, unix_path):
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind((listen_host, listen_port))
    server.listen()
    print(f"bridge listening {listen_host}:{listen_port} -> {unix_path}", flush=True)

    while True:
        client, _ = server.accept()
        upstream = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        upstream.connect(unix_path)
        pid = os.fork()
        if pid == 0:
            server.close()
            raw_proxy(client, upstream)
            os._exit(0)
        client.close()
        upstream.close()


def hci_proxy(vhci_fd, connection):
    os.set_blocking(vhci_fd, False)
    selector = selectors.DefaultSelector()
    selector.register(vhci_fd, selectors.EVENT_READ, "vhci")
    selector.register(connection, selectors.EVENT_READ, "sock")
    vhci_buf = b""
    sock_buf = b""

    while True:
        for key, _ in selector.select():
            if key.data == "vhci":
                try:
                    data = os.read(vhci_fd, 4096)
                except BlockingIOError:
                    continue
                if not data:
                    return
                vhci_buf += data
                packets, vhci_buf = take_h4_packets(vhci_buf)
                for packet in packets:
                    if packet[:1] != b"\xff":
                        connection.sendall(packet)
            else:
                try:
                    data = connection.recv(4096)
                except BlockingIOError:
                    continue
                if not data:
                    return
                sock_buf += data
                packets, sock_buf = take_h4_packets(sock_buf)
                for packet in packets:
                    if packet[:1] != b"\xff":
                        write_all_fd(vhci_fd, packet)


def main():
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    client = subparsers.add_parser("client")
    client.add_argument("host")
    client.add_argument("--port", type=int, default=45550)
    client.add_argument("--connect-timeout", type=float, default=10)

    bridge_parser = subparsers.add_parser("bridge")
    bridge_parser.add_argument("--listen-host", default="127.0.0.1")
    bridge_parser.add_argument("--port", type=int, default=45550)
    bridge_parser.add_argument("--unix-path", default="/tmp/bt-server-le")

    args = parser.parse_args()
    if args.command == "bridge":
        bridge(args.listen_host, args.port, args.unix_path)
        return

    hci_proxy(open_vhci(), connect_tcp(args.host, args.port, args.connect_timeout))


if __name__ == "__main__":
    main()
