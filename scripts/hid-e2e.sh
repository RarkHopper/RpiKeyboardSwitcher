#!/usr/bin/env bash
set -euo pipefail

central_host="${KBD_E2E_CENTRAL_HOST:-10.0.2.2}"
central_port="${KBD_E2E_CENTRAL_PORT:-45560}"
vagrant_provider="${KBD_E2E_VAGRANT_PROVIDER:-utm}"
vagrant_cmd="${VAGRANT:-vagrant}"

log() {
  printf 'hid-e2e: %s\n' "$*"
}

fail() {
  printf 'hid-e2e: %s\n' "$*" >&2
  print_logs >&2 || true
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

vm_sudo() {
  local vm="$1"
  "${vagrant_cmd}" ssh "$vm" -c "sudo bash -s"
}

print_logs() {
  for vm in central peripheral; do
    printf '\n===== %s logs =====\n' "$vm"
    "${vagrant_cmd}" ssh "$vm" -c 'sudo bash -s' <<'REMOTE' || true
for file in \
  /tmp/hid-e2e-events.log \
  /tmp/hid-e2e-reader.log \
  /tmp/btmon-report.log \
  /tmp/bluez-agent.log \
  /tmp/kbd-hid.log \
  /tmp/hidraw-cuse.log \
  /tmp/bluetoothd.log \
  /tmp/btvirt.log \
  /tmp/hci-bridge.log \
  /tmp/hci-client.log \
  /tmp/btmgmt.log \
  /tmp/bluetoothctl-pair.log; do
  if [ -s "$file" ]; then
    printf '\n--- %s ---\n' "$file"
    tail -200 "$file"
  fi
done
REMOTE
  done
}

start_vms() {
  need_command "${vagrant_cmd}"
  log "starting Vagrant VMs"
  "${vagrant_cmd}" up --provider="${vagrant_provider}" central peripheral
}

reset_bluetooth_host() {
  local vm="$1"
  vm_sudo "$vm" <<'REMOTE'
set -euo pipefail

systemctl stop bluetooth.service bluetooth.target >/dev/null 2>&1 || true
systemctl mask --runtime bluetooth.service >/dev/null 2>&1 || true
systemctl stop bluetooth.service bluetooth.target >/dev/null 2>&1 || true
pkill -x bluetoothctl >/dev/null 2>&1 || true
pkill -x bluetoothd >/dev/null 2>&1 || true
pkill -x btvirt >/dev/null 2>&1 || true
pkill -x btmon >/dev/null 2>&1 || true
pkill -x kbd-hid >/dev/null 2>&1 || true
pkill -x hidraw-cuse >/dev/null 2>&1 || true
pkill -f '(^|[ /])hci-proxy\.py( |$)' >/dev/null 2>&1 || true
pkill -f '(^|[ /])bluez-agent\.py( |$)' >/dev/null 2>&1 || true
pkill -f '(^|[ /])bluez-pair\.py( |$)' >/dev/null 2>&1 || true
sleep 1
rmmod hci_vhci >/dev/null 2>&1 || true
modprobe hci_vhci
rm -rf /var/lib/bluetooth/*
REMOTE
}

start_bluez_adapter() {
  local vm="$1"
  vm_sudo "$vm" <<'REMOTE'
set -euo pipefail

for _ in $(seq 1 100); do
  [ -d /sys/class/bluetooth/hci0 ] && break
  sleep 0.1
done
[ -d /sys/class/bluetooth/hci0 ]

if command -v bluetoothd >/dev/null 2>&1; then
  bluetoothd_path="$(command -v bluetoothd)"
else
  bluetoothd_path="/usr/libexec/bluetooth/bluetoothd"
fi
"$bluetoothd_path" -n -d >/tmp/bluetoothd.log 2>&1 &

for _ in $(seq 1 100); do
  busctl --system get-property org.bluez /org/bluez/hci0 org.bluez.Adapter1 Address >/tmp/bluez-adapter.log 2>&1 &&
    break
  sleep 0.1
done
busctl --system get-property org.bluez /org/bluez/hci0 org.bluez.Adapter1 Address >/tmp/bluez-adapter.log

btmgmt_cmd() {
  {
    printf 'select 0\n'
    printf '%s\n' "$1"
    printf 'quit\n'
  } | script -qfec btmgmt /dev/null >>/tmp/btmgmt.log 2>&1
}

btmgmt_cmd 'power off'
btmgmt_cmd 'le on'
btmgmt_cmd 'bredr off'
btmgmt_cmd 'power on'
btmgmt_cmd 'connectable on'
REMOTE
}

start_central() {
  log "starting central Bluetooth host"
  reset_bluetooth_host central
  vm_sudo central <<'REMOTE'
set -euo pipefail

rm -f /tmp/hid-e2e-events.log /tmp/hid-e2e-reader.log /tmp/bluetoothctl-pair.log \
  /tmp/bluez-agent.log /tmp/bluetoothd.log /tmp/btvirt.log /tmp/hci-bridge.log \
  /tmp/hci-client.log /tmp/btmgmt.log

rm -f /tmp/bt-server-le
btvirt -s >/tmp/btvirt.log 2>&1 &
tools_uv() {
  UV_PROJECT_ENVIRONMENT=/home/vagrant/.cache/rpi-keyboard-switcher-tools/.venv \
    uv --project /vagrant/tools --directory /vagrant/tools run \
    --locked --managed-python --python 3.12 --extra runtime --no-dev "$@"
}

for _ in $(seq 1 100); do
  [ -S /tmp/bt-server-le ] && break
  sleep 0.1
done
[ -S /tmp/bt-server-le ]

tools_uv python hci-proxy.py bridge \
  --listen-host 0.0.0.0 \
  --port 45550 \
  --unix-path /tmp/bt-server-le >/tmp/hci-bridge.log 2>&1 &
tools_uv python hci-proxy.py client 127.0.0.1 --port 45550 >/tmp/hci-client.log 2>&1 &
REMOTE

  start_bluez_adapter central

  vm_sudo central <<'REMOTE'
set -euo pipefail
tools_uv() {
  UV_PROJECT_ENVIRONMENT=/home/vagrant/.cache/rpi-keyboard-switcher-tools/.venv \
    uv --project /vagrant/tools --directory /vagrant/tools run \
    --locked --managed-python --python 3.12 --extra runtime --no-dev "$@"
}
tools_uv python bluez-agent.py --capability KeyboardDisplay >/tmp/bluez-agent.log 2>&1 &
for _ in $(seq 1 50); do
  grep -q '^agent registered ' /tmp/bluez-agent.log 2>/dev/null && break
  sleep 0.1
done
grep -q '^agent registered ' /tmp/bluez-agent.log
REMOTE
}

start_peripheral() {
  log "starting peripheral BLE keyboard"
  reset_bluetooth_host peripheral
  vm_sudo peripheral <<REMOTE
set -euo pipefail

rm -f /tmp/hidraw.path /tmp/send-report /tmp/kbd-e2e.yaml /tmp/kbd-hid.log \
  /tmp/hidraw-cuse.log /tmp/bluetoothd.log /tmp/hci-client.log /tmp/btmgmt.log

modprobe cuse
UV_PROJECT_ENVIRONMENT=/home/vagrant/.cache/rpi-keyboard-switcher-tools/.venv \
  uv --project /vagrant/tools --directory /vagrant/tools run \
  --locked --managed-python --python 3.12 --extra runtime --no-dev \
  python hci-proxy.py client "${central_host}" --port "${central_port}" >/tmp/hci-client.log 2>&1 &
REMOTE

  start_bluez_adapter peripheral

  vm_sudo peripheral <<'REMOTE'
set -euo pipefail
cd /vagrant
GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod /usr/local/go/bin/go build -o /tmp/kbd-hid ./cmd/kbd-hid
cflags="$(pkg-config fuse3 --cflags)"
libs="$(pkg-config fuse3 --libs)"
cc -Wall -Wextra -O2 -o /tmp/hidraw-cuse ./tools/hidraw-cuse.c $cflags $libs -pthread

/tmp/hidraw-cuse --name rpi-hidraw-e2e --path-file /tmp/hidraw.path --trigger-file /tmp/send-report >/tmp/hidraw-cuse.log 2>&1 &
for _ in $(seq 1 50); do
  [ -s /tmp/hidraw.path ] && break
  sleep 0.1
done
hidraw_device="$(cat /tmp/hidraw.path)"

cat >/tmp/kbd-e2e.yaml <<YAML
behavior:
  disconnect_others: true
  reconnect_wait_sec: 0
hid:
  adapter: hci0
  name: Rpi Keyboard Switcher
  appearance: keyboard
  pairable: true
  discoverable: true
  hidraw_device: ${hidraw_device}
YAML

/tmp/kbd-hid daemon --config /tmp/kbd-e2e.yaml >/tmp/kbd-hid.log 2>&1 &
for _ in $(seq 1 100); do
  grep -q 'GATT application registered' /tmp/bluetoothd.log 2>/dev/null &&
    grep -q 'Advertisement registered' /tmp/bluetoothd.log 2>/dev/null &&
    break
  sleep 0.2
done
grep -q 'GATT application registered' /tmp/bluetoothd.log
grep -q 'Advertisement registered' /tmp/bluetoothd.log
REMOTE
}

peripheral_address() {
  vm_sudo peripheral <<'REMOTE' | awk '/^addr / { print $2; exit }'
set -euo pipefail
btmgmt info | awk '
  $1 == "hci0:" { found = 1; next }
  found && $1 == "addr" { print "addr " $2; exit }
'
REMOTE
}

pair_central() {
  local mac="$1"
  log "pairing central with ${mac}"
  vm_sudo central <<REMOTE
set -euo pipefail

{
  UV_PROJECT_ENVIRONMENT=/home/vagrant/.cache/rpi-keyboard-switcher-tools/.venv \
    uv --project /vagrant/tools --directory /vagrant/tools run \
    --locked --managed-python --python 3.12 --extra runtime --no-dev \
    python bluez-pair.py --adapter hci0 "${mac}"
} >/tmp/bluetoothctl-pair.log 2>&1

grep -q 'Paired: yes' /tmp/bluetoothctl-pair.log
grep -q 'Connected: yes' /tmp/bluetoothctl-pair.log
grep -q 'Trusted: yes' /tmp/bluetoothctl-pair.log
REMOTE
}

wait_for_central_input() {
  local mac_lower
  mac_lower="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
  log "waiting for central evdev keyboard"
  vm_sudo central <<REMOTE
set -euo pipefail

mac="${mac_lower}"
for _ in \$(seq 1 150); do
  event="\$(awk -v mac="\$mac" '
    \$0 == "U: Uniq=" mac { found = 1; next }
    found && /^H: Handlers=/ {
      for (i = 1; i <= NF; i++) {
        if (\$i ~ /^event/) {
          print \$i
          exit
        }
      }
    }
    /^$/ { found = 0 }
  ' /proc/bus/input/devices)"
  if [ -n "\$event" ] && [ -e "/dev/input/\$event" ]; then
    printf '%s\n' "/dev/input/\$event" >/tmp/hid-e2e-event.path
    sleep 3
    exit 0
  fi
  sleep 0.2
done
exit 1
REMOTE
}

capture_input() {
  log "capturing central evdev and hidraw"
  vm_sudo central <<'REMOTE'
set -euo pipefail

event_path="$(cat /tmp/hid-e2e-event.path)"
hidraw_path="$(find /sys/devices/virtual/misc/uhid -maxdepth 3 -type d -name 'hidraw*' | sort | tail -1)"
hidraw_path="/dev/$(basename "$hidraw_path")"

(timeout 25s btmon >/tmp/btmon-report.log 2>&1) &
UV_PROJECT_ENVIRONMENT=/home/vagrant/.cache/rpi-keyboard-switcher-tools/.venv \
  timeout 22s uv --project /vagrant/tools --directory /vagrant/tools run \
  --locked --managed-python --python 3.12 --no-dev \
  python - "$event_path" "$hidraw_path" >/tmp/hid-e2e-events.log 2>/tmp/hid-e2e-reader.log <<'PY' &
import binascii
import os
import select
import struct
import sys
import time

event_path = sys.argv[1]
hidraw_path = sys.argv[2]
event_fd = os.open(event_path, os.O_RDONLY | os.O_NONBLOCK)
hidraw_fd = os.open(hidraw_path, os.O_RDONLY | os.O_NONBLOCK)
fmt = "llHHI"
size = struct.calcsize(fmt)
end = time.time() + 21
print(f"ready event={event_path} hidraw={hidraw_path}", flush=True)

while time.time() < end:
    readable, _, _ = select.select([event_fd, hidraw_fd], [], [], 0.5)
    for fd in readable:
        data = os.read(fd, 4096)
        if fd == hidraw_fd:
            print(f"hidraw {binascii.hexlify(data).decode()}", flush=True)
            continue
        for offset in range(0, len(data) // size * size, size):
            _, _, event_type, code, value = struct.unpack(fmt, data[offset:offset + size])
            print(f"event type={event_type} code={code} value={value}", flush=True)
PY

for _ in $(seq 1 50); do
  grep -q '^ready ' /tmp/hid-e2e-events.log 2>/dev/null && exit 0
  sleep 0.1
done
exit 1
REMOTE
}

trigger_input() {
  log "triggering fake hidraw keyboard"
  vm_sudo peripheral <<'REMOTE'
set -euo pipefail
rm -f /tmp/send-report
touch /tmp/send-report
REMOTE
}

verify_input() {
  log "verifying central input events"
  vm_sudo central <<'REMOTE'
set -euo pipefail

for _ in $(seq 1 100); do
  grep -q 'event type=1 code=30 value=1' /tmp/hid-e2e-events.log 2>/dev/null &&
    grep -q 'event type=1 code=30 value=0' /tmp/hid-e2e-events.log 2>/dev/null &&
    grep -q 'hidraw 010000040000000000' /tmp/hid-e2e-events.log 2>/dev/null &&
    grep -q 'hidraw 010000000000000000' /tmp/hid-e2e-events.log 2>/dev/null &&
    exit 0
  sleep 0.1
done
exit 1
REMOTE
}

main() {
  start_vms
  start_central
  start_peripheral
  mac="$(peripheral_address)"
  [ -n "$mac" ] || fail "peripheral address was empty"
  pair_central "$mac"
  wait_for_central_input "$mac"
  capture_input
  trigger_input
  verify_input
  log "passed: virtual HCI pair, BLE HID notification, hidraw report, and evdev KEY_A press/release"
}

main "$@"
