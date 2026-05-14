# RpiKeyboardSwitcher

[日本語版](README.ja.md)

RpiKeyboardSwitcher is a Go prototype for using a Raspberry Pi as a Bluetooth HID keyboard bridge.
The Raspberry Pi advertises itself as a BLE keyboard, learns paired target PCs from BlueZ, caches them in its config, and later switches between those cached targets from a short `kbd` command over SSH.

USB keyboard input forwarding is not implemented yet. The current `kbd-hid` daemon can advertise a BLE HID keyboard and send fixed test text after the host subscribes to HID notifications.

## Commands

- `kbd`: runs on a PC and sends commands to the Raspberry Pi over SSH.
- `kbd-rpi`: runs on the Raspberry Pi and switches cached Bluetooth targets through `bluetoothctl`.
- `kbd-hid`: runs on the Raspberry Pi as a long-running BLE HID daemon.

## Deployment

| Machine | Install | Config | Role |
| --- | --- | --- | --- |
| Raspberry Pi | `kbd-rpi`, `kbd-hid` | `/etc/kbd-switch/config.yaml` | Advertises the BLE keyboard, caches confirmed Bluetooth targets, and switches targets. |
| PC used to run switch commands | `kbd` | `~/.config/kbd-switch/config.yaml` | Knows how to SSH to the Raspberry Pi. It does not store Bluetooth MAC addresses. |
| PC used as a keyboard target | nothing required for input | OS Bluetooth settings | Pairs with `Rpi Keyboard Switcher` as a normal BLE keyboard. Install `kbd` here only if this PC also runs switch commands. |
| Wired keyboard | none | none | Plugs into the Raspberry Pi over USB. Input forwarding is a later step. |

## Flow

First, learn a target on the Raspberry Pi:

```text
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
  -> pair/connect from the target PC in the OS Bluetooth settings
  -> the host subscribes to HID input notifications
  -> kbd-hid reads BlueZ Device1 Address and Alias/Name
  -> kbd-hid saves targets.<generated-name> in /etc/kbd-switch/config.yaml
```

Then switch to that target from a PC:

```text
kbd switch laptop
  -> ssh pi@rpi-kbd.local kbd-rpi switch laptop
  -> kbd-rpi reads targets.laptop.bluetooth_mac
  -> bluetoothctl connect <cached MAC>
```

The generated target key and display name are meant to be edited by the user. The Bluetooth MAC address is the cached connection information and should usually be left alone.

## Build

```sh
go build -o dist/kbd ./cmd/kbd
GOOS=linux GOARCH=arm64 go build -o dist/kbd-rpi ./cmd/kbd-rpi
GOOS=linux GOARCH=arm64 go build -o dist/kbd-hid ./cmd/kbd-hid
```

Use `GOARCH=arm` instead of `arm64` for a 32-bit Raspberry Pi OS install.

## PC Setup

Create the PC-side config:

```sh
mkdir -p ~/.config/kbd-switch
$EDITOR ~/.config/kbd-switch/config.yaml
```

Format:

```yaml
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
```

Fields:

- `rpi.host`: SSH host name or address for the Raspberry Pi.
- `rpi.user`: SSH user.
- `rpi.remote_command`: command name or path to run on the Raspberry Pi. It must not contain whitespace.

The PC config intentionally does not contain Bluetooth target addresses. `kbd list`, `kbd switch <target>`, and tab completion ask the Raspberry Pi over SSH.

Examples:

```sh
kbd list
kbd switch laptop
kbd laptop
kbd status
```

`kbd laptop` is a shorthand for `kbd switch laptop`. If a target has the same name as a command, use `kbd switch <target>`.

SSH keys, `ssh-agent`, `known_hosts`, and host aliases should be configured through OpenSSH. Set `KBD_CONFIG=/path/to/config.yaml` to use a different config path without passing `--config` every time.

## Raspberry Pi Setup

Create the Raspberry Pi config:

```sh
sudo install -d -m 0755 /etc/kbd-switch
sudoedit /etc/kbd-switch/config.yaml
```

Initial format:

```yaml
targets: {}

behavior:
  disconnect_others: true
  reconnect_wait_sec: 2

hid:
  adapter: hci0
  name: Rpi Keyboard Switcher
  appearance: keyboard
  pairable: true
  discoverable: true
```

After a target is learned, the file will contain entries like this:

```yaml
targets:
  laptop:
    name: Work Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
```

Fields:

- `targets`: targets accepted by `kbd-rpi switch <target>`.
- `targets.<target>.name`: human-readable target name shown by `kbd-rpi list`.
- `targets.<target>.bluetooth_mac`: Bluetooth MAC address learned from BlueZ. Use uppercase `AA:BB:CC:DD:EE:FF` format.
- `behavior.disconnect_others`: when true or omitted, disconnect the previous state entry before connecting another target.
- `behavior.reconnect_wait_sec`: seconds to wait after disconnecting the previous target. Use `0` to skip waiting.
- `hid.adapter`: BlueZ adapter name used by `kbd-hid`, usually `hci0`.
- `hid.name`: BLE device name shown in the host Bluetooth settings.
- `hid.appearance`: HID appearance. Currently only `keyboard` is supported.
- `hid.pairable`: when true or omitted, allow incoming pairing requests.
- `hid.discoverable`: when true or omitted, make the adapter discoverable.

Target names may contain only letters, digits, `_`, `-`, and `.`. Unknown YAML fields are rejected.

## Learn A Target

Stop the systemd service if it is already running and you want to use `--test-text` for the first check:

```sh
sudo systemctl stop kbd-hid.service
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
```

On the target PC, open the OS Bluetooth settings and pair with `Rpi Keyboard Switcher`. Once the host subscribes to HID notifications, `kbd-hid` reads the connected BlueZ device and adds it to `targets`. If the same Bluetooth MAC address is already present, the existing target key and name are kept.

Inspect the BLE settings without touching Bluetooth:

```sh
kbd-hid inspect --config /etc/kbd-switch/config.yaml
```

## Switch Targets

On the Raspberry Pi:

```sh
kbd-rpi list
kbd-rpi switch laptop
kbd-rpi status
kbd-rpi disconnect
```

From a PC with `kbd` installed:

```sh
kbd list
kbd laptop
kbd status
```

The default Raspberry Pi state path is `/run/kbd-switch/state.json`. Set `KBD_RPI_CONFIG=/path/to/config.yaml` to use a different Raspberry Pi config path.

## systemd

Install the Raspberry Pi binaries and systemd unit:

```sh
sudo install -D -m 0755 dist/kbd-rpi /usr/local/bin/kbd-rpi
sudo install -D -m 0755 dist/kbd-hid /usr/local/bin/kbd-hid
sudo install -D -m 0644 rpi/systemd/kbd-hid.service /etc/systemd/system/kbd-hid.service
sudo systemctl daemon-reload
sudo systemctl enable --now kbd-hid.service
```

Check daemon logs:

```sh
journalctl -u kbd-hid.service -f
```

## Tab Completion

For zsh:

```sh
eval "$(kbd completion zsh)"
eval "$(kbd-rpi completion zsh)"
```

For bash:

```sh
eval "$(kbd completion bash)"
eval "$(kbd-rpi completion bash)"
```

Completion candidates are read on each completion request. `kbd` asks the Raspberry Pi over SSH, and `kbd-rpi` reads Raspberry Pi `targets`.

## Security

The Raspberry Pi sits in the key input path. A compromised or untrusted Raspberry Pi could read, store, modify, or inject key input. Do not use this with a work PC or managed PC without approval from the owner or administrator.

Input logging is not part of the initial implementation and should stay off by default.

## Development

```sh
make fmt
make check
```
