package input

import (
	"sort"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidreport"
)

const DefaultKeyboardGlob = "/dev/input/by-id/*-event-kbd"

type KeyboardState struct {
	modifiers byte
	keys      map[byte]bool
}

func (state *KeyboardState) Apply(code uint16, value int32) (hidreport.Report, bool) {
	if value == 2 {
		return hidreport.Report{}, false
	}
	if value != 0 && value != 1 {
		return hidreport.Report{}, false
	}

	if mask, ok := modifierForCode(code); ok {
		before := state.modifiers
		if value == 1 {
			state.modifiers |= mask
		} else {
			state.modifiers &^= mask
		}

		return state.Report(), before != state.modifiers
	}

	key, ok := keyForCode(code)
	if !ok {
		return hidreport.Report{}, false
	}
	if state.keys == nil {
		state.keys = map[byte]bool{}
	}

	before := state.keys[key]
	if value == 1 {
		state.keys[key] = true
	} else {
		delete(state.keys, key)
	}
	if before == (value == 1) {
		return hidreport.Report{}, false
	}

	return state.Report(), true
}

func (state *KeyboardState) Reset() (hidreport.Report, bool) {
	if state.modifiers == 0 && len(state.keys) == 0 {
		return hidreport.Report{}, false
	}

	state.modifiers = 0
	clear(state.keys)

	return state.Report(), true
}

func (state KeyboardState) Report() hidreport.Report {
	report := hidreport.Report{state.modifiers}
	keys := make([]int, 0, len(state.keys))
	for code := range state.keys {
		keys = append(keys, int(code))
	}
	sort.Ints(keys)

	index := 2
	for _, code := range keys {
		if index >= len(report) {
			break
		}
		report[index] = byte(code)
		index++
	}

	return report
}

func modifierForCode(code uint16) (byte, bool) {
	switch code {
	case 29:
		return 0x01, true
	case 42:
		return 0x02, true
	case 56:
		return 0x04, true
	case 125:
		return 0x08, true
	case 97:
		return 0x10, true
	case 54:
		return 0x20, true
	case 100:
		return 0x40, true
	case 126:
		return 0x80, true
	default:
		return 0x00, false
	}
}

func keyForCode(code uint16) (byte, bool) {
	keys := map[uint16]byte{
		1:   0x29,
		2:   0x1e,
		3:   0x1f,
		4:   0x20,
		5:   0x21,
		6:   0x22,
		7:   0x23,
		8:   0x24,
		9:   0x25,
		10:  0x26,
		11:  0x27,
		12:  0x2d,
		13:  0x2e,
		14:  0x2a,
		15:  0x2b,
		16:  0x14,
		17:  0x1a,
		18:  0x08,
		19:  0x15,
		20:  0x17,
		21:  0x1c,
		22:  0x18,
		23:  0x0c,
		24:  0x12,
		25:  0x13,
		26:  0x2f,
		27:  0x30,
		28:  0x28,
		30:  0x04,
		31:  0x16,
		32:  0x07,
		33:  0x09,
		34:  0x0a,
		35:  0x0b,
		36:  0x0d,
		37:  0x0e,
		38:  0x0f,
		39:  0x33,
		40:  0x34,
		41:  0x35,
		43:  0x31,
		44:  0x1d,
		45:  0x1b,
		46:  0x06,
		47:  0x19,
		48:  0x05,
		49:  0x11,
		50:  0x10,
		51:  0x36,
		52:  0x37,
		53:  0x38,
		55:  0x55,
		57:  0x2c,
		58:  0x39,
		59:  0x3a,
		60:  0x3b,
		61:  0x3c,
		62:  0x3d,
		63:  0x3e,
		64:  0x3f,
		65:  0x40,
		66:  0x41,
		67:  0x42,
		68:  0x43,
		69:  0x53,
		70:  0x47,
		71:  0x5f,
		72:  0x60,
		73:  0x61,
		74:  0x56,
		75:  0x5c,
		76:  0x5d,
		77:  0x5e,
		78:  0x57,
		79:  0x59,
		80:  0x5a,
		81:  0x5b,
		82:  0x62,
		83:  0x63,
		86:  0x64,
		87:  0x44,
		88:  0x45,
		96:  0x58,
		98:  0x54,
		99:  0x46,
		102: 0x4a,
		103: 0x52,
		104: 0x4b,
		105: 0x50,
		106: 0x4f,
		107: 0x4d,
		108: 0x51,
		109: 0x4e,
		110: 0x49,
		111: 0x4c,
		119: 0x48,
	}

	key, ok := keys[code]
	return key, ok
}
