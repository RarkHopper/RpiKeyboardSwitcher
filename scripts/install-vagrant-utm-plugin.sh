#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plugin_rel="third_party/vagrant_utm"
plugin_dir="${repo_root}/${plugin_rel}"
vagrant_cmd="${VAGRANT:-vagrant}"

log() {
  printf 'vagrant-utm-plugin: %s\n' "$*"
}

fail() {
  printf 'vagrant-utm-plugin: %s\n' "$*" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

need_command "${vagrant_cmd}"
need_command git

if [ ! -f "${plugin_dir}/vagrant_utm.gemspec" ]; then
  log "initializing ${plugin_rel}"
  git -C "${repo_root}" submodule update --init --recursive "${plugin_rel}"
fi

[ -f "${plugin_dir}/vagrant_utm.gemspec" ] ||
  fail "missing ${plugin_rel}; run git submodule update --init --recursive ${plugin_rel}"

if [ -x /opt/vagrant/embedded/bin/gem ]; then
  gem_cmd="/opt/vagrant/embedded/bin/gem"
else
  need_command gem
  gem_cmd="gem"
fi

log "building vagrant_utm gem"
build_output="$(cd "${plugin_dir}" && "${gem_cmd}" build vagrant_utm.gemspec)"
printf '%s\n' "${build_output}"

gem_file="$(printf '%s\n' "${build_output}" | awk '/File:/ { print $2; exit }')"
[ -n "${gem_file}" ] || fail "could not find built gem path"

gem_path="${plugin_dir}/${gem_file}"
[ -f "${gem_path}" ] || fail "built gem was not found: ${gem_path}"

log "installing project-local Vagrant plugin from ${plugin_rel}/${gem_file}"
"${vagrant_cmd}" plugin install --local "${gem_path}"
