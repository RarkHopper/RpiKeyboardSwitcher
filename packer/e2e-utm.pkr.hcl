packer {
  required_plugins {
    utm = {
      version = "= 4.0.0"
      source  = "github.com/naveenrajm7/utm"
    }
  }
}

locals {
  box_output        = "${path.root}/../dist/boxes/rpi-keyboard-switcher-e2e-utm.box"
  build_output      = "${path.root}/../dist/packer/rpi-keyboard-switcher-e2e-utm"
  cloud_image_url   = "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-arm64.img"
  cloud_image_sha   = "1ea801e659d2f5035ac294e0faab0aac9b6ba66753df933ba5c7beab0c689bd0"
  cloud_init_source = "${path.root}/cloud-init"
  tools_stage       = "/tmp/rpi-keyboard-switcher-tools"
}

source "utm-cloud" "e2e" {
  iso_url      = local.cloud_image_url
  iso_checksum = "sha256:${local.cloud_image_sha}"

  vm_name            = "RpiKeyboardSwitcher-E2E-Base"
  vm_arch            = "aarch64"
  cpus               = 2
  memory             = 4096
  output_directory   = local.build_output
  hypervisor         = true
  resize_cloud_image = true
  uefi_boot          = true
  display_nopause    = true
  boot_nopause       = true
  export_nopause     = true

  use_cd   = true
  cd_label = "cidata"
  cd_files = [
    "${local.cloud_init_source}/meta-data",
    "${local.cloud_init_source}/network-config",
    "${local.cloud_init_source}/user-data",
  ]

  ssh_username = "vagrant"
  ssh_password = "vagrant"
  ssh_timeout  = "10m"

  shutdown_command = "echo 'vagrant' | sudo -S /sbin/halt -h -p"
}

build {
  name = "rpi-keyboard-switcher-e2e-utm"

  sources = [
    "source.utm-cloud.e2e",
  ]

  provisioner "shell" {
    inline = [
      "mkdir -p ${local.tools_stage}",
    ]
  }

  provisioner "file" {
    source      = "${path.root}/../tools/pyproject.toml"
    destination = "${local.tools_stage}/pyproject.toml"
  }

  provisioner "file" {
    source      = "${path.root}/../tools/uv.lock"
    destination = "${local.tools_stage}/uv.lock"
  }

  provisioner "shell" {
    execute_command = "echo 'vagrant' | {{ .Vars }} sudo -S -E bash '{{ .Path }}'"
    script          = "${path.root}/../scripts/provision-e2e-vm.sh"
  }

  post-processor "utm-vagrant" {
    compression_level = 9
    output            = local.box_output
  }
}
