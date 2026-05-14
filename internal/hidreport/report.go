package hidreport

import "fmt"

const modifierLeftShift byte = 0x02

type Report [8]byte

func ReportsForText(text string) ([]Report, error) {
	reports := make([]Report, 0, len(text)*2)
	for _, char := range text {
		report, err := PressReport(char)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report, ReleaseReport())
	}

	return reports, nil
}

func PressReport(char rune) (Report, error) {
	modifier, keycode, ok := keyForRune(char)
	if !ok {
		return Report{}, fmt.Errorf("unsupported HID test character: %q", char)
	}

	return Report{modifier, 0x00, keycode}, nil
}

func ReleaseReport() Report {
	return Report{}
}

func (report Report) Bytes() []byte {
	return []byte{
		report[0],
		report[1],
		report[2],
		report[3],
		report[4],
		report[5],
		report[6],
		report[7],
	}
}

func Bytes(reports []Report) [][]byte {
	out := make([][]byte, 0, len(reports))
	for _, report := range reports {
		out = append(out, report.Bytes())
	}

	return out
}

func keyForRune(char rune) (byte, byte, bool) {
	if char >= 'a' && char <= 'z' {
		return 0x00, byte(char-'a') + 0x04, true
	}
	if char >= 'A' && char <= 'Z' {
		return modifierLeftShift, byte(char-'A') + 0x04, true
	}
	if char >= '1' && char <= '9' {
		return 0x00, byte(char-'1') + 0x1e, true
	}

	switch char {
	case '0':
		return 0x00, 0x27, true
	case '\n', '\r':
		return 0x00, 0x28, true
	case '\t':
		return 0x00, 0x2b, true
	case ' ':
		return 0x00, 0x2c, true
	case '-':
		return 0x00, 0x2d, true
	case '_':
		return modifierLeftShift, 0x2d, true
	case '=':
		return 0x00, 0x2e, true
	case '+':
		return modifierLeftShift, 0x2e, true
	case '[':
		return 0x00, 0x2f, true
	case '{':
		return modifierLeftShift, 0x2f, true
	case ']':
		return 0x00, 0x30, true
	case '}':
		return modifierLeftShift, 0x30, true
	case '\\':
		return 0x00, 0x31, true
	case '|':
		return modifierLeftShift, 0x31, true
	case ';':
		return 0x00, 0x33, true
	case ':':
		return modifierLeftShift, 0x33, true
	case '\'':
		return 0x00, 0x34, true
	case '"':
		return modifierLeftShift, 0x34, true
	case '`':
		return 0x00, 0x35, true
	case '~':
		return modifierLeftShift, 0x35, true
	case ',':
		return 0x00, 0x36, true
	case '<':
		return modifierLeftShift, 0x36, true
	case '.':
		return 0x00, 0x37, true
	case '>':
		return modifierLeftShift, 0x37, true
	case '/':
		return 0x00, 0x38, true
	case '?':
		return modifierLeftShift, 0x38, true
	case '!':
		return modifierLeftShift, 0x1e, true
	case '@':
		return modifierLeftShift, 0x1f, true
	case '#':
		return modifierLeftShift, 0x20, true
	case '$':
		return modifierLeftShift, 0x21, true
	case '%':
		return modifierLeftShift, 0x22, true
	case '^':
		return modifierLeftShift, 0x23, true
	case '&':
		return modifierLeftShift, 0x24, true
	case '*':
		return modifierLeftShift, 0x25, true
	case '(':
		return modifierLeftShift, 0x26, true
	case ')':
		return modifierLeftShift, 0x27, true
	default:
		return 0x00, 0x00, false
	}
}
