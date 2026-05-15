#!/usr/bin/env bash
set -euo pipefail

adapter="${KBD_E2E_ADAPTER:-hci0}"
central_adapter="${KBD_E2E_CENTRAL_ADAPTER:-hci1}"
adapter_index="${adapter#hci}"
central_adapter_index="${central_adapter#hci}"
device_name="${KBD_E2E_NAME:-Rpi Keyboard Switcher}"
test_text="${KBD_E2E_TEXT:-a}"
repo_dir="${KBD_E2E_REPO:-/work}"

btvirt_pid=""
bluetoothd_pid=""
hid_pid=""

log() {
  printf 'hid-e2e: %s\n' "$*"
}

fail() {
  printf 'hid-e2e: %s\n' "$*" >&2
  print_logs
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

cleanup() {
  set +e
  for pid in "$hid_pid" "$bluetoothd_pid" "$btvirt_pid"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null
      wait "$pid" 2>/dev/null
    fi
  done
}
trap cleanup EXIT

print_logs() {
  set +e
  for file in /tmp/kbd-hid.log /tmp/bluetoothd.log /tmp/btvirt.log /tmp/btmgmt-peripheral.log /tmp/btmgmt-central.log /tmp/bluetoothctl-scan.log /tmp/bluetoothctl-connect.log /tmp/bluetoothctl-gatt.log; do
    if [ -s "$file" ]; then
      printf '\n===== %s =====\n' "$file" >&2
      tail -200 "$file" >&2
    fi
  done
}

check_prerequisites() {
  need_command btvirt
  need_command bluetoothctl
  need_command btmgmt
  need_command dbus-daemon
  need_command go
  need_command modprobe

  case "$adapter_index:$central_adapter_index" in
    *[!0-9:]* | :* | *:)
      fail "KBD_E2E_ADAPTER and KBD_E2E_CENTRAL_ADAPTER must look like hci0 and hci1"
      ;;
  esac

  log "kernel: $(uname -r)"
  log "checking hci_vhci and uhid"

  if ! modprobe hci_vhci >/tmp/modprobe-hci-vhci.log 2>&1; then
    cat /tmp/modprobe-hci-vhci.log >&2
    fail "hci_vhci is unavailable; use a Linux VM whose kernel has CONFIG_BT_HCIVHCI"
  fi
  if ! modprobe uhid >/tmp/modprobe-uhid.log 2>&1; then
    cat /tmp/modprobe-uhid.log >&2
    fail "uhid is unavailable; use a Linux VM whose kernel has CONFIG_UHID"
  fi

  [ -e /dev/vhci ] || fail "/dev/vhci was not created after loading hci_vhci"
  [ -e /dev/uhid ] || fail "/dev/uhid was not created after loading uhid"

  log "kernel prerequisites are present"
}

start_system_bus() {
  mkdir -p /run/dbus
  rm -f /run/dbus/system_bus_socket /run/dbus/pid
  dbus-daemon --system --fork
  export DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket
}

wait_for_path() {
  path="$1"
  name="$2"
  for _ in $(seq 1 50); do
    [ -e "$path" ] && return 0
    sleep 0.1
  done
  fail "$name did not appear: $path"
}

wait_for_bluetoothctl() {
  for _ in $(seq 1 50); do
    if bluetoothctl list >/tmp/bluetoothctl-list.log 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  cat /tmp/bluetoothctl-list.log >&2 || true
  fail "bluetoothctl could not talk to bluetoothd"
}

bluetoothd_path() {
  if command -v bluetoothd >/dev/null 2>&1; then
    command -v bluetoothd
    return
  fi
  for path in /usr/lib/bluetooth/bluetoothd /usr/libexec/bluetooth/bluetoothd; do
    if [ -x "$path" ]; then
      printf '%s\n' "$path"
      return
    fi
  done
  fail "missing bluetoothd"
}

start_bluez_lab() {
  start_system_bus

  btvirt -l2 -L >/tmp/btvirt.log 2>&1 &
  btvirt_pid="$!"
  wait_for_path "/sys/class/bluetooth/$adapter" "$adapter"
  wait_for_path "/sys/class/bluetooth/$central_adapter" "$central_adapter"

  "$(bluetoothd_path)" -n -E >/tmp/bluetoothd.log 2>&1 &
  bluetoothd_pid="$!"
  wait_for_bluetoothctl

  btmgmt --index "$adapter_index" power off >/tmp/btmgmt-peripheral.log 2>&1 || true
  btmgmt --index "$central_adapter_index" power off >/tmp/btmgmt-central.log 2>&1 || true
  btmgmt --index "$adapter_index" le on >>/tmp/btmgmt-peripheral.log 2>&1 || true
  btmgmt --index "$central_adapter_index" le on >>/tmp/btmgmt-central.log 2>&1 || true
  btmgmt --index "$adapter_index" bredr off >>/tmp/btmgmt-peripheral.log 2>&1 || true
  btmgmt --index "$central_adapter_index" bredr off >>/tmp/btmgmt-central.log 2>&1 || true
  btmgmt --index "$adapter_index" power on >>/tmp/btmgmt-peripheral.log 2>&1 || true
  btmgmt --index "$central_adapter_index" power on >>/tmp/btmgmt-central.log 2>&1 || true
}

write_config() {
  cat >/tmp/kbd-e2e.yaml <<YAML
behavior:
  disconnect_others: true
  reconnect_wait_sec: 0
hid:
  adapter: ${adapter}
  name: ${device_name}
  appearance: keyboard
  pairable: true
  discoverable: true
YAML
}

build_kbd_hid() {
  cd "$repo_dir"
  GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod go build -o /tmp/kbd-hid ./cmd/kbd-hid
}

start_kbd_hid() {
  write_config
  build_kbd_hid
  /tmp/kbd-hid daemon --config /tmp/kbd-e2e.yaml --test-text "$test_text" >/tmp/kbd-hid.log 2>&1 &
  hid_pid="$!"
}

scan_for_device() {
  bluetoothctl --timeout 10 >/tmp/bluetoothctl-scan.log 2>&1 <<EOF || true
select ${central_adapter}
scan on
EOF

  awk -v name="$device_name" '
    $0 ~ name {
      for (i = 1; i <= NF; i++) {
        if ($i ~ /^[0-9A-F][0-9A-F](:[0-9A-F][0-9A-F]){5}$/) {
          print $i
          exit
        }
      }
    }
  ' /tmp/bluetoothctl-scan.log
}

connect_device() {
  mac="$1"
  bluetoothctl --timeout 20 >/tmp/bluetoothctl-connect.log 2>&1 <<EOF || true
select ${central_adapter}
agent KeyboardDisplay
default-agent
pair ${mac}
trust ${mac}
connect ${mac}
EOF
}

find_report_attribute() {
  mac="$1"
  bluetoothctl --timeout 10 >/tmp/bluetoothctl-gatt.log 2>&1 <<EOF || true
select ${central_adapter}
menu gatt
list-attributes ${mac}
EOF

  awk '
    /00002a4d-0000-1000-8000-00805f9b34fb|00002a22-0000-1000-8000-00805f9b34fb/ {
      for (i = 1; i <= NF; i++) {
        if ($i ~ /^\/org\/bluez\//) {
          print $i
          exit
        }
      }
    }
  ' /tmp/bluetoothctl-gatt.log
}

subscribe_report() {
  attribute="$1"
  bluetoothctl --timeout 10 >>/tmp/bluetoothctl-gatt.log 2>&1 <<EOF || true
select ${central_adapter}
menu gatt
select-attribute ${attribute}
notify on
EOF
}

wait_for_cached_peer() {
  for _ in $(seq 1 50); do
    if grep -q 'bluetooth_mac:' /tmp/kbd-e2e.yaml; then
      return 0
    fi
    sleep 0.2
  done
  fail "HID daemon did not cache a connected peer"
}

run_e2e() {
  check_prerequisites
  start_bluez_lab
  start_kbd_hid

  mac="$(scan_for_device)"
  [ -n "$mac" ] || fail "central adapter did not discover ${device_name}"
  log "discovered ${device_name} as ${mac}"

  connect_device "$mac"
  attribute="$(find_report_attribute "$mac")"
  [ -n "$attribute" ] || fail "HID report characteristic was not discovered"
  log "subscribing to ${attribute}"

  subscribe_report "$attribute"
  wait_for_cached_peer

  log "passed: advertisement, connection, GATT discovery, notification subscription, and peer caching"
}

case "${1:-check}" in
  check)
    check_prerequisites
    ;;
  run)
    run_e2e
    ;;
  *)
    fail "usage: hid-e2e [check|run]"
    ;;
esac
