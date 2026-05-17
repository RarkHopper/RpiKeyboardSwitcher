# Tools

このディレクトリには、Vagrant の Bluetooth HID E2E で使う Python/C 補助ツールを置いています。

## Python 環境

Python tools はこのディレクトリ内の `uv` 設定で管理します。

```sh
uv --project tools --directory tools sync --managed-python --python 3.12
```

DBus/GLib の実行時依存は必要な時だけ extra で入れます。

```sh
uv --project tools --directory tools sync --managed-python --python 3.12 --extra runtime
```

## ネイティブ依存

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

## チェック

プロジェクトルートから実行します。

```sh
make python-check
make python-runtime-check
```

`python-check` は Ruff、mypy、Pyright、`compileall` を実行します。`tools/stubs` のローカル stub で、このスクリプトが使う DBus と GLib の API を明示しているため、`dbus` や `gi` の import が解決できない状態を無視しません。

`python-runtime-check` は runtime extra 経由で `dbus`、`gi`、`GLib` を import し、DBus/GLib の開発ファイルが入っていることを確認します。
