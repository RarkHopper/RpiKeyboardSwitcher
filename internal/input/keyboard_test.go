package input_test

import (
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidreport"
	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/input"
)

func TestLinuxキー入力をHIDレポートへ変換する(t *testing.T) {
	var state input.KeyboardState

	report, ok := state.Apply(30, 1)
	if !ok {
		t.Fatal("a press changed = false, want true")
	}
	if want := (hidreport.Report{0x00, 0x00, 0x04}); !reflect.DeepEqual(report, want) {
		t.Fatalf("a press report = %#v, want %#v", report, want)
	}

	report, ok = state.Apply(30, 0)
	if !ok {
		t.Fatal("a release changed = false, want true")
	}
	if want := (hidreport.Report{}); !reflect.DeepEqual(report, want) {
		t.Fatalf("a release report = %#v, want %#v", report, want)
	}
}

func Test修飾キーと通常キーを同じレポートへ入れる(t *testing.T) {
	var state input.KeyboardState

	if _, ok := state.Apply(42, 1); !ok {
		t.Fatal("left shift press changed = false, want true")
	}
	report, ok := state.Apply(30, 1)
	if !ok {
		t.Fatal("a press changed = false, want true")
	}
	if want := (hidreport.Report{0x02, 0x00, 0x04}); !reflect.DeepEqual(report, want) {
		t.Fatalf("shift+a report = %#v, want %#v", report, want)
	}
}

func Testキーリピートは無視する(t *testing.T) {
	var state input.KeyboardState

	if _, ok := state.Apply(30, 1); !ok {
		t.Fatal("a press changed = false, want true")
	}
	if _, ok := state.Apply(30, 2); ok {
		t.Fatal("repeat changed = true, want false")
	}
}

func Test同期落ちでは解放レポートを返す(t *testing.T) {
	var state input.KeyboardState

	if _, ok := state.Apply(30, 1); !ok {
		t.Fatal("a press changed = false, want true")
	}
	report, ok := state.Reset()
	if !ok {
		t.Fatal("reset changed = false, want true")
	}
	if want := (hidreport.Report{}); !reflect.DeepEqual(report, want) {
		t.Fatalf("reset report = %#v, want %#v", report, want)
	}
}
