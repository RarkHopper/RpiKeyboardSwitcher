#!/usr/bin/env bash
set -euo pipefail

packer_cmd="${PACKER:-packer}"
git_cmd="${GIT:-git}"
go_cmd="${GO:-go}"
plugin_source="${PACKER_UTM_PLUGIN_SOURCE:-github.com/naveenrajm7/utm}"
plugin_repo="${PACKER_UTM_PLUGIN_REPO:-https://github.com/naveenrajm7/packer-plugin-utm.git}"
plugin_version="${PACKER_UTM_PLUGIN_VERSION:-4.0.0}"
plugin_tag="${PACKER_UTM_PLUGIN_TAG:-v${plugin_version}}"

log() {
  printf 'packer-utm-plugin: %s\n' "$*"
}

fail() {
  printf 'packer-utm-plugin: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

need_command "${packer_cmd}"
need_command "${git_cmd}"
need_command "${go_cmd}"
need_command osacompile
need_command patch
need_command perl

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/rpi-keyboard-switcher-packer-utm.XXXXXX")"
trap 'rm -rf "${work_dir}"' EXIT

src_dir="${work_dir}/packer-plugin-utm"
bin_path="${work_dir}/packer-plugin-utm-bin"

log "cloning ${plugin_repo} ${plugin_tag}"
"${git_cmd}" clone --depth 1 --branch "${plugin_tag}" "${plugin_repo}" "${src_dir}"

log "fixing malformed AppleScript continuation bytes"
perl -0pi -e 's/\xC2(?!\xAC)/\xC2\xAC/g' "${src_dir}"/builder/utm/common/scripts/*.applescript
osacompile -o "${work_dir}/create_vm.scpt" "${src_dir}/builder/utm/common/scripts/create_vm.applescript"
osacompile -o "${work_dir}/add_port_forwards.scpt" "${src_dir}/builder/utm/common/scripts/add_port_forwards.applescript"

log "patching Packer SSH network setup"
(
  cd "${src_dir}"
  patch -p1 <<'PATCH'
diff --git a/builder/utm/common/step_port_forwarding.go b/builder/utm/common/step_port_forwarding.go
index a9e68a2..b4791da 100644
--- a/builder/utm/common/step_port_forwarding.go
+++ b/builder/utm/common/step_port_forwarding.go
@@ -75,24 +75,9 @@ func (s *StepPortForwarding) Run(ctx context.Context, state multistep.StateBag)
 				return multistep.ActionHalt
 			}
 
-			// We now hard code interfaces as needed by Vagrant and Packer.
-			// 0 index - 'Shared Network' interface
-			// 1 index - 'Emulated VLAN' interface
-			// but this should be configurable
-
-			// Add access to localhost => UTM 'Shared Network' interface
-			if _, err := driver.ExecuteOsaScript("add_network_interface.applescript", vmId, "ShRd"); err != nil {
-				err := fmt.Errorf("error adding network interface: %s", err)
-				state.Put("error", err)
-				ui.Error(err.Error())
-				return multistep.ActionHalt
-			}
-
-			// TODO: check if we need to add the 'Shared Network' interface
-			// TODO: check if we need to add the 'Emulated VLAN' interface
-			// and then add if needed
-			// Make sure to configure the network interface to 'Emulated VLAN' mode
-			// required for port forwarding now in packer , later in vagrant
+			// Use one user-mode network interface while building the box.
+			// The Ubuntu cloud image brings its first NIC up with DHCP, and
+			// UTM host port forwarding is attached to this interface.
 			if _, err := driver.ExecuteOsaScript("add_network_interface.applescript", vmId, "EmUd"); err != nil {
 				err := fmt.Errorf("error adding network interface: %s", err)
 				state.Put("error", err)
@@ -106,7 +91,7 @@ func (s *StepPortForwarding) Run(ctx context.Context, state multistep.StateBag)
 		ui.Say(fmt.Sprintf("Creating forwarded port mapping for communicator (SSH, WinRM, etc) (host port %d)", commHostPort))
 		command := []string{
 			"add_port_forwards.applescript", vmId,
-			"--index", "1",
+			"--index", "0",
 			fmt.Sprintf("TcPp,,%d,127.0.0.1,%d", guestPort, commHostPort),
 		}
 		if _, err := driver.ExecuteOsaScript(command...); err != nil {
PATCH
)

log "building ${plugin_source} ${plugin_version}"
(
  cd "${src_dir}"
  "${go_cmd}" build \
    -ldflags "-s -w -X github.com/naveenrajm7/packer-plugin-utm/version.Version=${plugin_version} -X github.com/naveenrajm7/packer-plugin-utm/version.VersionPrerelease=" \
    -o "${bin_path}"
)

log "installing patched plugin"
"${packer_cmd}" plugins install --force --path "${bin_path}" "${plugin_source}"
