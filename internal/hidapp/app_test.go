package hidapp_test

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/bluez"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/config"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidapp"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/input"
)

type fakeDaemon struct {
	options bluez.DaemonOptions
	err     error
	called  bool
}

func (daemon *fakeDaemon) Run(_ context.Context, options bluez.DaemonOptions) error {
	daemon.called = true
	daemon.options = options

	return daemon.err
}

type fakeInput struct {
	descriptor input.Descriptor
	reports    []input.Report
}

func (fake fakeInput) Descriptor() (input.Descriptor, error) {
	return fake.descriptor, nil
}

func (fake fakeInput) Run(_ context.Context, send func(input.Report) error) error {
	for _, report := range fake.reports {
		if err := send(report); err != nil {
			return err
		}
	}

	return nil
}

func TestHIDCLIはdaemonで設定からBLEkeyboardを起動する(t *testing.T) {
	configPath := writeConfig(t)
	daemon := &fakeDaemon{}

	code := hidapp.App{
		Daemon: daemon,
		Input:  fakeInput{descriptor: testDescriptor()},
		Stderr: &bytes.Buffer{},
	}.Run([]string{"daemon", "--config", configPath})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if daemon.options.Adapter != "hci1" {
		t.Fatalf("adapter = %q, want hci1", daemon.options.Adapter)
	}
	if daemon.options.Name != "Desk Bridge" {
		t.Fatalf("name = %q, want Desk Bridge", daemon.options.Name)
	}
	if daemon.options.Appearance != bluez.KeyboardAppearance {
		t.Fatalf("appearance = %#v, want %#v", daemon.options.Appearance, bluez.KeyboardAppearance)
	}
	if !daemon.options.Pairable {
		t.Fatal("pairable = false, want true")
	}
	if !daemon.options.Discoverable {
		t.Fatal("discoverable = false, want true")
	}
	if !reflect.DeepEqual(daemon.options.ReportMap, testReportMap) {
		t.Fatalf("ReportMap = %#v, want %#v", daemon.options.ReportMap, testReportMap)
	}
	if !reflect.DeepEqual(daemon.options.InputReportIDs, []byte{0x02}) {
		t.Fatalf("InputReportIDs = %#v, want [2]", daemon.options.InputReportIDs)
	}
	if !reflect.DeepEqual(daemon.options.OutputReportIDs, []byte{0x03}) {
		t.Fatalf("OutputReportIDs = %#v, want [3]", daemon.options.OutputReportIDs)
	}
	if daemon.options.OnPeerReady == nil {
		t.Fatal("OnPeerReady is nil")
	}
	if daemon.options.InputReports == nil {
		t.Fatal("InputReports is nil")
	}
}

func TestHIDCLIはUSBキーボード入力をBLEreportへ渡す(t *testing.T) {
	configPath := writeConfig(t)
	daemon := &fakeDaemon{}
	wantReport := bluez.InputReport{ID: 0x02, Data: []byte{0x00, 0x00, 0x04}}

	code := hidapp.App{
		Daemon: daemon,
		Input: fakeInput{
			descriptor: testDescriptor(),
			reports:    []input.Report{{ID: wantReport.ID, Data: wantReport.Data}},
		},
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "daemon"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	var gotReports []bluez.InputReport
	err := daemon.options.InputReports(context.Background(), func(report bluez.InputReport) error {
		gotReports = append(gotReports, bluez.InputReport{ID: report.ID, Data: append([]byte(nil), report.Data...)})
		return nil
	})
	if err != nil {
		t.Fatalf("InputReports err = %v, want nil", err)
	}
	if !reflect.DeepEqual(gotReports, []bluez.InputReport{wantReport}) {
		t.Fatalf("reports = %#v, want %#v", gotReports, []bluez.InputReport{wantReport})
	}
}

func TestHIDCLIはinspectで実際に使うBLE設定を出す(t *testing.T) {
	configPath := writeConfig(t)
	stdout := &bytes.Buffer{}

	code := hidapp.App{
		Input:  fakeInput{descriptor: testDescriptor()},
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "inspect"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	for _, want := range []string{
		"adapter: hci1\n",
		"name: Desk Bridge\n",
		"appearance: keyboard (0x03C1)\n",
		"hidraw_device: /dev/hidraw0\n",
		"report_map_bytes: 15\n",
		"input_report_ids: 0x02\n",
		"service_uuid: " + bluez.HIDServiceUUID + "\n",
	} {
		if !bytes.Contains(stdout.Bytes(), []byte(want)) {
			t.Fatalf("stdout = %q, want to contain %q", stdout.String(), want)
		}
	}
}

func TestHIDCLIはBluetooth疎通後にtargetを設定へ保存する(t *testing.T) {
	configPath := writeConfig(t)
	daemon := &fakeDaemon{}

	code := hidapp.App{
		Daemon: daemon,
		Input:  fakeInput{descriptor: testDescriptor()},
		Stderr: &bytes.Buffer{},
	}.Run([]string{"--config", configPath, "daemon"})

	if code != 0 {
		t.Fatalf("終了コード = %d, want 0", code)
	}
	if err := daemon.options.OnPeerReady(bluez.Peer{
		Name:         "Work Laptop",
		BluetoothMAC: "AA:BB:CC:DD:EE:02",
	}); err != nil {
		t.Fatalf("OnPeerReady err = %v, want nil", err)
	}

	cfg, err := config.LoadRPI(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Targets["work-laptop"]
	want := config.Target{Name: "Work Laptop", BluetoothMAC: "AA:BB:CC:DD:EE:02"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("target = %#v, want %#v", got, want)
	}
}

func writeConfig(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/config.yaml"
	content := []byte(`
behavior:
  disconnect_others: true
  reconnect_wait_sec: 0
hid:
  adapter: hci1
  name: Desk Bridge
  appearance: keyboard
  pairable: true
  discoverable: true
  hidraw_device: /dev/hidraw0
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

var testReportMap = []byte{
	0x05, 0x01,
	0x09, 0x06,
	0xa1, 0x01,
	0x85, 0x02,
	0x81, 0x02,
	0x85, 0x03,
	0x91, 0x02,
	0xc0,
}

func testDescriptor() input.Descriptor {
	return input.Descriptor{
		ReportMap:       testReportMap,
		InputReportIDs:  []byte{0x02},
		OutputReportIDs: []byte{0x03},
		UsesReportID:    true,
	}
}
