# RpiKeyboardSwitcher

[English](README.md)

RpiKeyboardSwitcher は、Raspberry Pi を Bluetooth HID キーボードの橋渡しとして使うための Go 製プロトタイプです。Raspberry Pi が BLE キーボードとして広告し、接続できたPCを BlueZ から読み取り、設定ファイルへ保存します。以後は PC 側の短い `kbd` コマンドから SSH 経由で Raspberry Pi に切替を指示します。

`kbd-hid` デーモンは BLE HID キーボードとして広告し、ホストが HID 通知を有効にした後に Raspberry Pi の Linux 入力デバイスから読んだキー入力を送ります。初回確認用に固定のテスト文字も送れます。

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
| 有線キーボード | なし | なし | USB で Raspberry Pi に接続します。`kbd-hid` が Linux 入力デバイスからキー入力を読みます。 |

## 処理の流れ

まず Raspberry Pi に接続先を覚えさせます。

```text
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
  -> 対象PCのOS Bluetooth設定からペアリング/接続
  -> ホストが HID 入力通知を有効にする
  -> kbd-hid が BlueZ Device1 の Address と Alias/Name を読む
  -> /etc/kbd-switch/config.yaml に targets.<生成名> を保存
  -> Raspberry Pi の USB キーボード入力を BLE HID report として送る
```

その後、PC 側から切り替えます。

```text
kbd switch laptop
  -> ssh pi@rpi-kbd.local kbd-rpi switch laptop
  -> kbd-rpi が targets.laptop.bluetooth_mac を読む
  -> bluetoothctl connect <保存済みMAC>
```

自動生成された接続先名と表示名は、後から `/etc/kbd-switch/config.yaml` で変更できます。Bluetooth MAC アドレスは接続情報なので、通常はそのままにします。

## セットアップ

ここでは、開発PCでビルドしてから Raspberry Pi と切替コマンドを打つPCへ配置する流れで進めます。例では Raspberry Pi の SSH 接続先を `pi@rpi-kbd.local` とします。

### 0. 配線

- USB キーボードを Raspberry Pi の USB ポートに挿します。
- Raspberry Pi は電源を入れ、Bluetooth を有効にします。
- キーボード入力を受けるPCでは、Bluetooth 設定画面とテキストエディタなどの入力欄を開けるようにしておきます。
- 切替コマンドを打つPCから Raspberry Pi へ SSH できるようにしておきます。このPCは、キーボード入力を受けるPCと同じでも別でも構いません。

### 1. ビルド

開発PCで3つのバイナリを作ります。

```sh
make build
```

既定では、PC 側の `kbd` は開発PCと同じ OS/CPU 向け、Raspberry Pi 側の `kbd-rpi` と `kbd-hid` は 64-bit Raspberry Pi OS 向けに `linux/arm64` で作ります。開発PCとは別のPCで `kbd` を使う場合は、そのPC上で `make build` を実行するか、`LOCAL_GOOS` と `LOCAL_GOARCH` をそのPCに合わせて指定します。

32-bit Raspberry Pi OS 向けに作る場合は `RPI_GOARCH=arm` を渡します。

```sh
RPI_GOARCH=arm make build
```


### 2. Raspberry Pi へ配置

開発PCから Raspberry Pi へ `kbd-rpi`、`kbd-hid`、systemd unit を送ります。

```sh
scp dist/kbd-rpi dist/kbd-hid rpi/systemd/kbd-hid.service pi@rpi-kbd.local:/tmp/
ssh pi@rpi-kbd.local
```

ここから systemd の起動までは、Raspberry Pi のシェルで実行します。

Raspberry Pi では BlueZ と SSH を使います。Bluetooth アダプタ名は通常 `hci0` です。

```sh
command -v bluetoothctl
sudo systemctl enable --now bluetooth.service
systemctl is-active bluetooth.service
bluetoothctl list
ls /sys/class/bluetooth
```

`bluetoothctl` がない場合は Raspberry Pi 側に BlueZ を入れます。

```sh
sudo apt-get update
sudo apt-get install -y bluez
```

`bluetoothctl list` または `ls /sys/class/bluetooth` で `hci0` が出ない場合は、Raspberry Pi 側で Bluetooth が無効になっていないかを先に確認します。

`kbd-hid` は入力デバイスを読み取り、実行中は対象キーボードを Raspberry Pi の通常入力から外します。手動確認では `sudo kbd-hid ...` で実行し、systemd unit も root で起動します。

バイナリを `/usr/local/bin` に置きます。

```sh
sudo install -D -m 0755 /tmp/kbd-rpi /usr/local/bin/kbd-rpi
sudo install -D -m 0755 /tmp/kbd-hid /usr/local/bin/kbd-hid
```

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
  input_devices: []
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
- `hid.input_devices`: 読み取る Linux 入力デバイス。空または未指定なら `/dev/input/by-id/*-event-kbd` を使います。特定のキーボードだけ読む場合は `/dev/input/by-id/...-event-kbd` を指定します。

接続先名に使える文字は英数字、`_`、`-`、`.` だけです。未知の YAML フィールドはエラーにします。

### 3. BLE HID の設定を確認

Bluetooth に触る前に、`kbd-hid` が読む設定を確認します。

```sh
kbd-hid inspect --config /etc/kbd-switch/config.yaml
```

USB キーボードを Raspberry Pi に挿し、入力デバイス名を確認します。

```sh
ls -l /dev/input/by-id/*-event-kbd
```

複数のキーボードがあり、読む対象を固定したい場合は `hid.input_devices` に書きます。

```yaml
hid:
  input_devices:
    - /dev/input/by-id/usb-Example_Keyboard-event-kbd
```

### 4. 接続先を覚えさせる

初回は systemd ではなく手動で起動し、対象PCとのペアリングとテスト入力を確認します。

```sh
sudo systemctl stop kbd-hid.service 2>/dev/null || true
sudo kbd-hid daemon --config /etc/kbd-switch/config.yaml --test-text a
```

このコマンドは起動したまま待ちます。対象PCでテキストエディタなどの入力欄を開いてから、OS Bluetooth設定で `Rpi Keyboard Switcher` とペアリングします。ホストが HID 通知を有効にすると、`kbd-hid` が BlueZ の接続済みデバイスを読み取り、`targets` に保存します。同じ Bluetooth MAC アドレスがすでに保存済みなら、既存の接続先名と表示名を保ちます。

対象PCで `a` が入力され、Raspberry Pi 側の `/etc/kbd-switch/config.yaml` に `targets` が増えたら、USB キーボードの入力も対象PCへ届くことを確認します。確認後は `Ctrl-C` で止めます。接続先名や表示名は、この時点で編集できます。Bluetooth MAC アドレスは通常そのままにします。

### 5. systemd で常駐させる

手動起動で確認できたら、`kbd-hid` を systemd で起動します。

```sh
sudo install -D -m 0644 /tmp/kbd-hid.service /etc/systemd/system/kbd-hid.service
sudo systemctl daemon-reload
sudo systemctl enable --now kbd-hid.service
sudo journalctl -u kbd-hid.service -f
```

ログを確認できたら `Ctrl-C` で `journalctl` だけを止めます。`kbd-hid.service` は動き続けます。

### 6. 切替コマンドを打つPCへ配置

Raspberry Pi のシェルから `exit` で戻ります。開発PCを切替コマンドを打つPCとして使う場合は、`kbd` を PATH の通った場所に置きます。

```sh
mkdir -p ~/.local/bin
install -m 0755 dist/kbd ~/.local/bin/kbd
```

`~/.local/bin` に PATH が通っていない場合は、PATH に追加するか、別の場所に置きます。

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

SSH 鍵、`ssh-agent`、`known_hosts`、ホスト別設定は OpenSSH 側で設定します。`KBD_CONFIG=/path/to/config.yaml` を設定すると、毎回 `--config` を渡さずに別の設定ファイルを使えます。

### 7. 切り替えを確認

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
kbd switch laptop
kbd laptop
kbd status
```

`kbd laptop` は `kbd switch laptop` の短縮形です。接続先名がコマンド名と同じ場合は `kbd switch <target>` を使います。

Raspberry Pi 側の state の既定パスは `/run/kbd-switch/state.json` です。`KBD_RPI_CONFIG=/path/to/config.yaml` を設定すると、Raspberry Pi 側で別の設定ファイルを使えます。

## 補完

切替コマンドを打つPCで `kbd` の補完を読み込みます。

zsh:

```sh
eval "$(kbd completion zsh)"
```

bash:

```sh
eval "$(kbd completion bash)"
```

Raspberry Pi 側で `kbd-rpi` を直接使う場合は、Raspberry Pi のシェルで `kbd-rpi` の補完を読み込みます。

zsh:

```sh
eval "$(kbd-rpi completion zsh)"
```

bash:

```sh
eval "$(kbd-rpi completion bash)"
```

補完候補は補完のたびに読みます。`kbd` は SSH 経由で Raspberry Pi に問い合わせ、`kbd-rpi` は Raspberry Pi 側の `targets` を読みます。

## セキュリティ

Raspberry Pi はキー入力の経路上に置かれます。信頼できない Raspberry Pi を使うと、キー入力の読み取り、変更、注入が可能になります。業務PCや管理対象PCでは、所有者または管理者の許可なしに使わないでください。

## 開発

```sh
make fmt
make check
```
