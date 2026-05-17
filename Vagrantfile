GO_VERSION = "1.26.3"
GO_LINUX_ARM64_SHA256 = "9d89a3ea57d141c2b22d70083f2c8459ba3890f2d9e818e7e933b75614936565"
UV_VERSION = "0.9.22"
UV_LINUX_ARM64_SHA256 = "2f8716c407d5da21b8a3e8609ed358147216aaab28b96b1d6d7f48e9bcc6254e"

def provision_e2e_vm(config)
  config.vm.synced_folder ".", "/vagrant"
  config.vm.provision "shell", privileged: true, inline: <<-SHELL
    set -eu

    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y --no-install-recommends \
      bluez \
      bluez-test-tools \
      build-essential \
      ca-certificates \
      curl \
      dbus \
      git \
      gobject-introspection \
      kmod \
      libcairo2-dev \
      libdbus-1-dev \
      libfuse3-dev \
      libgirepository1.0-dev \
      libgirepository-2.0-dev \
      pkg-config \
      procps \
      "linux-modules-extra-$(uname -r)"

    tmp_dir="$(mktemp -d)"
    trap 'rm -rf "$tmp_dir"' EXIT

    go_archive="go#{GO_VERSION}.linux-arm64.tar.gz"
    curl -fsSL "https://go.dev/dl/${go_archive}" -o "${tmp_dir}/${go_archive}"
    printf '%s  %s\n' '#{GO_LINUX_ARM64_SHA256}' "${tmp_dir}/${go_archive}" | sha256sum -c -
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "${tmp_dir}/${go_archive}"
    ln -sf /usr/local/go/bin/go /usr/local/bin/go
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

    uv_archive="uv-aarch64-unknown-linux-gnu.tar.gz"
    curl -fsSL "https://github.com/astral-sh/uv/releases/download/#{UV_VERSION}/${uv_archive}" -o "${tmp_dir}/${uv_archive}"
    printf '%s  %s\n' '#{UV_LINUX_ARM64_SHA256}' "${tmp_dir}/${uv_archive}" | sha256sum -c -
    tar -C "$tmp_dir" -xzf "${tmp_dir}/${uv_archive}"
    install -m 0755 "${tmp_dir}/uv-aarch64-unknown-linux-gnu/uv" /usr/local/bin/uv
    install -m 0755 "${tmp_dir}/uv-aarch64-unknown-linux-gnu/uvx" /usr/local/bin/uvx

    install -d -m 0755 /opt/rpi-keyboard-switcher-tools
    UV_PROJECT_ENVIRONMENT=/opt/rpi-keyboard-switcher-tools/.venv \
      /usr/local/bin/uv --project /vagrant/tools --directory /vagrant/tools sync \
      --locked --managed-python --python 3.12 --extra runtime --no-dev

    cat >/etc/profile.d/go.sh <<'PROFILE'
export PATH=/usr/local/go/bin:$PATH
PROFILE
    chmod 0644 /etc/profile.d/go.sh
    git config --global --add safe.directory /vagrant
    sudo -u vagrant git config --global --add safe.directory /vagrant

    cat >/etc/modules-load.d/rpi-keyboard-switcher-e2e.conf <<'MODULES'
hci_vhci
cuse
MODULES
    modprobe hci_vhci
    modprobe cuse
    test -e /dev/vhci
    test -e /dev/cuse
  SHELL
end

def configure_utm(vm, name)
  vm.vm.provider "utm" do |utm|
    utm.name = name
    utm.cpus = 2
    utm.memory = 4096
    utm.directory_share_mode = "virtFS"
  end
end

Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-24.04"
  config.vm.box_architecture = "arm64"

  config.vm.define "central" do |central|
    central.vm.hostname = "rpi-keyboard-switcher-central"
    central.vm.network "forwarded_port", guest: 45550, host: 45560, auto_correct: false
    configure_utm(central, "RpiKeyboardSwitcher E2E Central")
    provision_e2e_vm(central)
  end

  config.vm.define "peripheral" do |peripheral|
    peripheral.vm.hostname = "rpi-keyboard-switcher-peripheral"
    configure_utm(peripheral, "RpiKeyboardSwitcher E2E Peripheral")
    provision_e2e_vm(peripheral)
  end
end
