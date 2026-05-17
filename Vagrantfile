GO_VERSION = "1.26.3"
GO_LINUX_ARM64_SHA256 = "9d89a3ea57d141c2b22d70083f2c8459ba3890f2d9e818e7e933b75614936565"

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
      kmod \
      libfuse3-dev \
      pkg-config \
      procps \
      python3 \
      "linux-modules-extra-$(uname -r)"

    go_archive="go#{GO_VERSION}.linux-arm64.tar.gz"
    if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go#{GO_VERSION}"; then
      tmp_dir="$(mktemp -d)"
      trap 'rm -rf "$tmp_dir"' EXIT
      curl -fsSL "https://go.dev/dl/${go_archive}" -o "${tmp_dir}/${go_archive}"
      printf '%s  %s\n' '#{GO_LINUX_ARM64_SHA256}' "${tmp_dir}/${go_archive}" | sha256sum -c -
      rm -rf /usr/local/go
      tar -C /usr/local -xzf "${tmp_dir}/${go_archive}"
      ln -sf /usr/local/go/bin/go /usr/local/bin/go
      ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
    fi

    printf 'export PATH=/usr/local/go/bin:$PATH\\n' >/etc/profile.d/go.sh
    chmod 0644 /etc/profile.d/go.sh
    git config --global --add safe.directory /vagrant
    sudo -u vagrant git config --global --add safe.directory /vagrant

    printf 'hci_vhci\\ncuse\\n' >/etc/modules-load.d/rpi-keyboard-switcher-e2e.conf
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
