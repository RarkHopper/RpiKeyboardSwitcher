# Tools

This directory contains the Python and C helpers used by the Vagrant Bluetooth HID E2E flow.

## Python Environment

Python tools are managed in this directory with `uv`.

```sh
uv --project tools --directory tools sync --managed-python --python 3.12
```

Runtime DBus/GLib dependencies are optional and are installed only when needed:

```sh
uv --project tools --directory tools sync --managed-python --python 3.12 --extra runtime
```

## Native Dependencies

Linux:

```sh
sudo apt-get install -y --no-install-recommends \
  build-essential \
  gobject-introspection \
  libcairo2-dev \
  libdbus-1-dev \
  libgirepository1.0-dev \
  libgirepository-2.0-dev \
  pkg-config
```

macOS:

```sh
brew install cairo dbus gobject-introspection pkg-config
```

## Checks

Run the Python checks from the project root:

```sh
make python-check
make python-runtime-check
```

`python-check` runs Ruff, mypy, Pyright, and `compileall`. The local stubs in `tools/stubs` describe the DBus and GLib APIs used by these scripts, so missing `dbus` or `gi` imports fail in type checking instead of being ignored.

`python-runtime-check` imports `dbus`, `gi`, and `GLib` through the runtime extra to verify the native DBus/GLib development files are installed.
