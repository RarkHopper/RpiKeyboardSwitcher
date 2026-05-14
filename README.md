# RpiKeyboardSwitcher

RpiKeyboardSwitcher is a Go CLI prototype for switching a Raspberry Pi based keyboard bridge between paired PCs.

Phase 1 provides two commands:

- `kbd`: runs on a PC and sends switch requests to the Raspberry Pi over SSH.
- `kbd-rpi`: runs on the Raspberry Pi and switches Bluetooth targets through `bluetoothctl`.

Bluetooth HID keyboard emulation and long-running input forwarding are not part of Phase 1.

## PC command

Create the local config first:

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

targets:
  pc1: desktop
  pc2: laptop
```

```sh
kbd switch pc1
kbd pc1
kbd status
kbd list
```

`kbd switch pc1` reads the local config, resolves `pc1` to a Raspberry Pi target, and runs:

```sh
ssh pi@rpi-kbd.local kbd-rpi switch desktop
```

`kbd pc1` is kept as a shorthand. If a target has the same name as a command, use `kbd switch <target>`.
SSH keys, `ssh-agent`, `known_hosts`, and host aliases should be configured through OpenSSH.
Set `KBD_CONFIG=/path/to/config.yaml` to use a different config path without passing `--config` every time.

Fields:

- `rpi.host`: SSH host name or address for the Raspberry Pi.
- `rpi.user`: SSH user.
- `rpi.remote_command`: command name or path to run on the Raspberry Pi. It must not contain whitespace.
- `targets`: local names accepted by `kbd switch <target>`. Each value is the Raspberry Pi device name passed to `kbd-rpi switch <device>`.

## Raspberry Pi command

Create the Raspberry Pi config first:

```sh
sudo install -d -m 0755 /etc/kbd-switch
sudoedit /etc/kbd-switch/config.yaml
```

Format:

```yaml
devices:
  desktop:
    name: Main Desktop
    bluetooth_mac: AA:BB:CC:DD:EE:01

  laptop:
    name: Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02

behavior:
  disconnect_others: true
  reconnect_wait_sec: 2
```

```sh
kbd-rpi switch laptop
kbd-rpi status
kbd-rpi disconnect
```

The default Raspberry Pi config path is `/etc/kbd-switch/config.yaml`.
The default state path is `/run/kbd-switch/state.json`.
Set `KBD_RPI_CONFIG=/path/to/config.yaml` to use a different Raspberry Pi config path.
Device names can match command names because devices are selected through `kbd-rpi switch <device>`.

Fields:

- `devices`: names accepted by `kbd-rpi switch <device>`.
- `devices.<device>.name`: human-readable device name shown by `kbd-rpi list`.
- `devices.<device>.bluetooth_mac`: Bluetooth MAC address. Use uppercase `AA:BB:CC:DD:EE:FF` format.
- `behavior.disconnect_others`: when true or omitted, disconnect the previous state entry before connecting another device.
- `behavior.reconnect_wait_sec`: seconds to wait after disconnecting the previous device. Use `0` to skip waiting.

Device names and local target names may contain only letters, digits, `_`, `-`, and `.`. Unknown YAML fields are rejected.

## Tab completion

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

Completion candidates are read from the YAML files on each completion request. If you edit the config, the next Tab reads the new target or device names. A custom config path is read from `KBD_CONFIG` / `KBD_RPI_CONFIG`, or from `--config <path>` when it appears before the command being completed.

## Development

```sh
make fmt
make check
```
