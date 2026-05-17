E2E_BOX_NAME = ENV.fetch("KBD_E2E_BOX_NAME", "rpi-keyboard-switcher/e2e-ubuntu-24.04-arm64")

def configure_e2e_vm(vm, name)
  vm.vm.synced_folder ".", "/vagrant"
  vm.vm.provider "utm" do |utm|
    utm.name = name
    utm.cpus = 2
    utm.memory = 4096
    utm.directory_share_mode = "virtFS"
  end
end

Vagrant.configure("2") do |config|
  config.vm.box = E2E_BOX_NAME
  config.vm.box_check_update = false

  config.vm.define "central" do |central|
    central.vm.hostname = "rpi-keyboard-switcher-central"
    central.vm.network "forwarded_port", guest: 45550, host: 45560, auto_correct: false
    configure_e2e_vm(central, "RpiKeyboardSwitcher E2E Central")
  end

  config.vm.define "peripheral" do |peripheral|
    peripheral.vm.hostname = "rpi-keyboard-switcher-peripheral"
    configure_e2e_vm(peripheral, "RpiKeyboardSwitcher E2E Peripheral")
  end
end
