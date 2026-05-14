# RpiKeyboardSwitcher

[English](README.md)

RpiKeyboardSwitcher は、Raspberry Pi を Bluetooth HID キーボードの橋渡しとして使うための Go 製プロトタイプです。Raspberry Pi が BLE キーボードとして広告し、接続できたPCを BlueZ から読み取り、設定ファイルへ保存します。以後は PC 側の短い `kbd` コマンドから SSH 経由で Raspberry Pi に切替を指示します。

USB キーボード入力の中継はまだ未実装です。現在の `kbd-hid` デーモンは BLE HID キーボードとして広告し、ホストが HID 通知を有効にした後に固定のテスト文字を送れます。

## コマンド

- `kbd`: PC 側で実行し、SSH 経由で Raspberry Pi へ指示します。
- `kbd-rpi`: Raspberry Pi 側で実行し、保存済みの Bluetooth 接続先を `bluetoothctl` で切り替えます。
- `kbd-hid`: Raspberry Pi 側で常駐し、BLE HID キーボードを広告します。

## 配置

| 機材 | 入れるもの | 設定 | 役割 |
| --- | --- | --- | --- |
| Raspberry Pi | `kbd-rpi`, `kbd-hid` | `/etc/kbd-switch/config.yaml` | BLE キーボードを広告し、疎通した Bluetooth 接続先を `targets` に保存し、切替を行います。 |
| 切替コマンドを打つPC | `kbd` | `~/.config/kbd-switch/config.yaml` | Raspberry Pi への SSH 接続方法だけを持ちます。Bluetooth MAC アドレスは持ちません。 |
| キーボード入力を受けるPC | 入力を受けるだけなら不要 | OS の Bluetooth 設定 | `Rpi Keyboard Switcher` とペアリングし、普通の BLE キーボードとして入力を受けます。このPCから切替も行うなら `kbd` も入れます。 |
| 有線キーボード | なし | なし | USB で Raspberry Pi に接続します。入力中継は次の段階です。 |

## 処理の流れ

まず Raspberry Pi に接続先を覚えさせます。

```text
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
  -> 対象PCのOS Bluetooth設定からペアリング/接続
  -> ホストが HID 入力通知を有効にする
  -> kbd-hid が BlueZ Device1 の Address と Alias/Name を読む
  -> /etc/kbd-switch/config.yaml に targets.<生成名> を保存
```

その後、PC 側から切り替えます。

```text
kbd switch laptop
  -> ssh pi@rpi-kbd.local kbd-rpi switch laptop
  -> kbd-rpi が targets.laptop.bluetooth_mac を読む
  -> bluetoothctl connect <保存済みMAC>
```

自動生成された接続先名と表示名は、後から `/etc/kbd-switch/config.yaml` で変更できます。Bluetooth MAC アドレスは接続情報なので、通常はそのままにします。

## ビルド

```sh
go build -o dist/kbd ./cmd/kbd
GOOS=linux GOARCH=arm64 go build -o dist/kbd-rpi ./cmd/kbd-rpi
GOOS=linux GOARCH=arm64 go build -o dist/kbd-hid ./cmd/kbd-hid
```

32-bit の Raspberry Pi OS へ入れる場合は `GOARCH=arm` を使います。

## PC 側設定

PC 側の設定ファイルを作ります。

```sh
mkdir -p ~/.config/kbd-switch
$EDITOR ~/.config/kbd-switch/config.yaml
```

形式:

```yaml
rpi:
  host: rpi-kbd.local
  user: pi
  remote_command: kbd-rpi
```

項目:

- `rpi.host`: Raspberry Pi の SSH ホスト名またはIPアドレス。
- `rpi.user`: SSH ユーザー。
- `rpi.remote_command`: Raspberry Pi で実行するコマンド名またはパス。空白は使えません。

PC 側の設定には Bluetooth 接続先の情報を書きません。`kbd list`、`kbd switch <target>`、補完候補の取得は、SSH 経由で Raspberry Pi に問い合わせます。

例:

```sh
kbd list
kbd switch laptop
kbd laptop
kbd status
```

`kbd laptop` は `kbd switch laptop` の短縮形です。接続先名がコマンド名と同じ場合は `kbd switch <target>` を使います。

SSH 鍵、`ssh-agent`、`known_hosts`、ホスト別設定は OpenSSH 側で設定します。`KBD_CONFIG=/path/to/config.yaml` を設定すると、毎回 `--config` を渡さずに別の設定ファイルを使えます。

## Raspberry Pi 側設定

Raspberry Pi 側の設定ファイルを作ります。

```sh
sudo install -d -m 0755 /etc/kbd-switch
sudoedit /etc/kbd-switch/config.yaml
```

初期形式:

```yaml
targets: {}

behavior:
  disconnect_others: true
  reconnect_wait_sec: 2

hid:
  adapter: hci0
  name: Rpi Keyboard Switcher
  appearance: keyboard
  pairable: true
  discoverable: true
```

接続先が保存されると、次のような項目が追加されます。

```yaml
targets:
  laptop:
    name: Work Laptop
    bluetooth_mac: AA:BB:CC:DD:EE:02
```

項目:

- `targets`: `kbd-rpi switch <target>` で指定できる接続先。
- `targets.<target>.name`: `kbd-rpi list` に表示する名前。
- `targets.<target>.bluetooth_mac`: BlueZ から読んだ Bluetooth MAC アドレス。`AA:BB:CC:DD:EE:FF` の大文字表記です。
- `behavior.disconnect_others`: true または未指定なら、新しい接続先へ接続する前に前回の接続先を切断します。
- `behavior.reconnect_wait_sec`: 前回の接続先を切断した後に待つ秒数。`0` なら待ちません。
- `hid.adapter`: `kbd-hid` が使う BlueZ アダプタ名。通常は `hci0` です。
- `hid.name`: ホスト側の Bluetooth 設定に表示される BLE デバイス名。
- `hid.appearance`: HID の appearance。現在は `keyboard` のみ対応しています。
- `hid.pairable`: true または未指定なら、ペアリング要求を受け付けます。
- `hid.discoverable`: true または未指定なら、アダプタを discoverable にします。

接続先名に使える文字は英数字、`_`、`-`、`.` だけです。未知の YAML フィールドはエラーにします。

## 接続先を覚えさせる

初回確認で `--test-text` を使う場合、すでに systemd サービスが動いていれば止めます。

```sh
sudo systemctl stop kbd-hid.service
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
```

対象PCのOS Bluetooth設定を開き、`Rpi Keyboard Switcher` とペアリングします。ホストが HID 通知を有効にすると、`kbd-hid` が BlueZ の接続済みデバイスを読み取り、`targets` に保存します。同じ Bluetooth MAC アドレスがすでに保存済みなら、既存の接続先名と表示名を保ちます。

Bluetooth に触らず、使われる BLE 設定だけ確認するには次を使います。

```sh
kbd-hid inspect --config /etc/kbd-switch/config.yaml
```

## 接続先を切り替える

Raspberry Pi 側:

```sh
kbd-rpi list
kbd-rpi switch laptop
kbd-rpi status
kbd-rpi disconnect
```

`kbd` を入れた PC 側:

```sh
kbd list
kbd laptop
kbd status
```

Raspberry Pi 側の state の既定パスは `/run/kbd-switch/state.json` です。`KBD_RPI_CONFIG=/path/to/config.yaml` を設定すると、Raspberry Pi 側で別の設定ファイルを使えます。

## systemd

Raspberry Pi 側のバイナリと systemd unit を入れます。

```sh
sudo install -D -m 0755 dist/kbd-rpi /usr/local/bin/kbd-rpi
sudo install -D -m 0755 dist/kbd-hid /usr/local/bin/kbd-hid
sudo install -D -m 0644 rpi/systemd/kbd-hid.service /etc/systemd/system/kbd-hid.service
sudo systemctl daemon-reload
sudo systemctl enable --now kbd-hid.service
```

ログを見るには次を使います。

```sh
journalctl -u kbd-hid.service -f
```

## 補完

zsh:

```sh
eval "$(kbd completion zsh)"
eval "$(kbd-rpi completion zsh)"
```

bash:

```sh
eval "$(kbd completion bash)"
eval "$(kbd-rpi completion bash)"
```

補完候補は補完のたびに読みます。`kbd` は SSH 経由で Raspberry Pi に問い合わせ、`kbd-rpi` は Raspberry Pi 側の `targets` を読みます。

## セキュリティ

Raspberry Pi はキー入力の経路上に置かれます。信頼できない Raspberry Pi を使うと、キー入力の読み取り、保存、変更、注入が可能になります。業務PCや管理対象PCでは、所有者または管理者の許可なしに使わないでください。

入力ログ保存は初期実装に含めず、標準では無効のままにします。

## 開発

```sh
make fmt
make check
```
