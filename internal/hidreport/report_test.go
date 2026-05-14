package hidreport_test

import (
	"reflect"
	"testing"

	"github.com/RarkHopper/RpiKeyboardSwitcher/internal/hidreport"
)

func TestASCII文字をHIDキーボードreportへ変換する(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []hidreport.Report
	}{
		{
			name: "小文字を押下と解放へ変換する",
			text: "a",
			want: []hidreport.Report{{0x00, 0x00, 0x04}, {}},
		},
		{
			name: "大文字はshift付きで変換する",
			text: "A",
			want: []hidreport.Report{{0x02, 0x00, 0x04}, {}},
		},
		{
			name: "数字を変換する",
			text: "1",
			want: []hidreport.Report{{0x00, 0x00, 0x1e}, {}},
		},
		{
			name: "空白を変換する",
			text: " ",
			want: []hidreport.Report{{0x00, 0x00, 0x2c}, {}},
		},
		{
			name: "改行をenterとして変換する",
			text: "\n",
			want: []hidreport.Report{{0x00, 0x00, 0x28}, {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hidreport.ReportsForText(tt.text)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("reports = %#v, want %#v", got, tt.want)
			}
		})
	}
}
