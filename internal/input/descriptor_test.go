package input_test

import (
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/input"
)

func TestHIDreportDescriptorからreportIDなしの入力reportを読む(t *testing.T) {
	descriptor, err := input.ParseDescriptor([]byte{
		0x05, 0x01,
		0x09, 0x06,
		0xa1, 0x01,
		0x81, 0x02,
		0x91, 0x02,
		0xc0,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if descriptor.UsesReportID {
		t.Fatal("UsesReportID = true, want false")
	}
	if !reflect.DeepEqual(descriptor.InputReportIDs, []byte{0x00}) {
		t.Fatalf("InputReportIDs = %#v, want [0]", descriptor.InputReportIDs)
	}
	if !reflect.DeepEqual(descriptor.OutputReportIDs, []byte{0x00}) {
		t.Fatalf("OutputReportIDs = %#v, want [0]", descriptor.OutputReportIDs)
	}

	report, ok := descriptor.Report([]byte{0x00, 0x00, 0x04})
	if !ok {
		t.Fatal("report ok = false, want true")
	}
	if want := (input.Report{ID: 0x00, Data: []byte{0x00, 0x00, 0x04}}); !reflect.DeepEqual(report, want) {
		t.Fatalf("report = %#v, want %#v", report, want)
	}
}

func TestHIDreportDescriptorからreportIDありの入力reportを読む(t *testing.T) {
	descriptor, err := input.ParseDescriptor([]byte{
		0x05, 0x01,
		0x09, 0x06,
		0xa1, 0x01,
		0x85, 0x02,
		0x81, 0x02,
		0x85, 0x03,
		0x81, 0x02,
		0x85, 0x04,
		0x91, 0x02,
		0xc0,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !descriptor.UsesReportID {
		t.Fatal("UsesReportID = false, want true")
	}
	if !reflect.DeepEqual(descriptor.InputReportIDs, []byte{0x02, 0x03}) {
		t.Fatalf("InputReportIDs = %#v, want [2 3]", descriptor.InputReportIDs)
	}
	if !reflect.DeepEqual(descriptor.OutputReportIDs, []byte{0x04}) {
		t.Fatalf("OutputReportIDs = %#v, want [4]", descriptor.OutputReportIDs)
	}

	report, ok := descriptor.Report([]byte{0x02, 0x00, 0x00, 0x04})
	if !ok {
		t.Fatal("report ok = false, want true")
	}
	if want := (input.Report{ID: 0x02, Data: []byte{0x00, 0x00, 0x04}}); !reflect.DeepEqual(report, want) {
		t.Fatalf("report = %#v, want %#v", report, want)
	}
	if _, ok := descriptor.Report([]byte{0x04, 0x00}); ok {
		t.Fatal("unknown report ID ok = true, want false")
	}
}

func Test壊れたHIDreportDescriptorは拒否する(t *testing.T) {
	if _, err := input.ParseDescriptor([]byte{0x75}); err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestHIDreportDescriptorはゼロのReportIDを拒否する(t *testing.T) {
	_, err := input.ParseDescriptor([]byte{
		0x05, 0x01,
		0x09, 0x06,
		0xa1, 0x01,
		0x85, 0x00,
		0x81, 0x02,
		0xc0,
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
}
