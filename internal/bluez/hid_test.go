package bluez

import (
	"context"
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

type emittedSignal struct {
	path   dbus.ObjectPath
	name   string
	values []any
}

type fakeEmitter struct {
	signals []emittedSignal
}

func (emitter *fakeEmitter) Emit(path dbus.ObjectPath, name string, values ...any) error {
	emitter.signals = append(emitter.signals, emittedSignal{
		path:   path,
		name:   name,
		values: append([]any(nil), values...),
	})

	return nil
}

func TestGATTのObjectManagerはHIDserviceとcharacteristicを返す(t *testing.T) {
	objects := NewHIDApplication().ManagedObjects()

	service, ok := objects[ServicePath][GATTServiceInterface]
	if !ok {
		t.Fatalf("HID service is missing: %#v", objects[ServicePath])
	}
	if got := service["UUID"].Value(); got != HIDServiceUUID {
		t.Fatalf("service UUID = %#v, want %#v", got, HIDServiceUUID)
	}

	for _, path := range []dbus.ObjectPath{
		HIDInfoPath,
		ReportMapPath,
		ControlPointPath,
		ProtocolModePath,
		ReportPath,
		BootInputPath,
		BootOutputPath,
	} {
		characteristic, ok := objects[path][GATTCharacteristicInterface]
		if !ok {
			t.Fatalf("%s characteristic is missing", path)
		}
		if got := characteristic["Service"].Value(); got != ServicePath {
			t.Fatalf("%s service path = %#v, want %#v", path, got, ServicePath)
		}
	}
}

func TestAdvertisementはHIDserviceとkeyboardのappearanceを含む(t *testing.T) {
	advertisement := NewHIDAdvertisement("Rpi Keyboard Switcher", KeyboardAppearance)
	properties := advertisement.Properties()

	if got := properties["Type"].Value(); got != "peripheral" {
		t.Fatalf("Type = %#v, want peripheral", got)
	}
	if got := properties["LocalName"].Value(); got != "Rpi Keyboard Switcher" {
		t.Fatalf("LocalName = %#v, want Rpi Keyboard Switcher", got)
	}
	if got := properties["Appearance"].Value(); got != KeyboardAppearance {
		t.Fatalf("Appearance = %#v, want %#v", got, KeyboardAppearance)
	}
	if got := properties["ServiceUUIDs"].Value(); !reflect.DeepEqual(got, []string{HIDServiceUUID}) {
		t.Fatalf("ServiceUUIDs = %#v, want %#v", got, []string{HIDServiceUUID})
	}
}

func Test通知開始後に押下reportと解放reportを順に送る(t *testing.T) {
	app := NewHIDApplication()
	emitter := &fakeEmitter{}
	app.SetEmitter(emitter)

	if err := app.characteristics[ReportPath].StartNotify(); err != nil {
		t.Fatalf("StartNotify err = %v, want nil", err)
	}

	press := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}
	release := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := app.SendReportsAfterSubscription(context.Background(), [][]byte{press, release}); err != nil {
		t.Fatalf("SendReportsAfterSubscription err = %v, want nil", err)
	}

	wantValues := [][]byte{press, release}
	if len(emitter.signals) != len(wantValues) {
		t.Fatalf("signals = %#v, want %d signals", emitter.signals, len(wantValues))
	}
	for index, signal := range emitter.signals {
		if signal.path != ReportPath {
			t.Fatalf("signal[%d].path = %s, want %s", index, signal.path, ReportPath)
		}
		if signal.name != PropertiesInterface+".PropertiesChanged" {
			t.Fatalf("signal[%d].name = %s, want PropertiesChanged", index, signal.name)
		}
		changed, ok := signal.values[1].(map[string]dbus.Variant)
		if !ok {
			t.Fatalf("signal[%d] changed properties = %#v", index, signal.values[1])
		}
		if got := changed["Value"].Value(); !reflect.DeepEqual(got, wantValues[index]) {
			t.Fatalf("signal[%d] Value = %#v, want %#v", index, got, wantValues[index])
		}
	}
}

func TestBootProtocolではBootInputへだけreportを送る(t *testing.T) {
	app := NewHIDApplication()
	emitter := &fakeEmitter{}
	app.SetEmitter(emitter)

	if err := app.characteristics[ReportPath].StartNotify(); err != nil {
		t.Fatalf("Report StartNotify err = %v, want nil", err)
	}
	if err := app.characteristics[BootInputPath].StartNotify(); err != nil {
		t.Fatalf("BootInput StartNotify err = %v, want nil", err)
	}
	if err := app.characteristics[ProtocolModePath].WriteValue([]byte{0x00}, map[string]dbus.Variant{}); err != nil {
		t.Fatalf("ProtocolMode WriteValue err = %v, want nil", err)
	}

	report := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := app.SendReportsAfterSubscription(context.Background(), [][]byte{report}); err != nil {
		t.Fatalf("SendReportsAfterSubscription err = %v, want nil", err)
	}

	if len(emitter.signals) != 1 {
		t.Fatalf("signals = %#v, want 1 signal", emitter.signals)
	}
	if emitter.signals[0].path != BootInputPath {
		t.Fatalf("signal path = %s, want %s", emitter.signals[0].path, BootInputPath)
	}
}
