package main

import (
	"image"
	"image/color"
	"testing"
)

func TestRenderGradient(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8(x * 255 / 7)})
		}
	}
	cells := render(img, 8, []byte(defaultRamp))
	if len(cells) != 2 { // 4 行 × (8/8) × 0.5 = 2
		t.Fatalf("行数 = %d, want 2", len(cells))
	}
	row := cells[0]
	if len(row) != 8 {
		t.Fatalf("列数 = %d, want 8", len(row))
	}
	if row[0].ch != ' ' {
		t.Errorf("最暗像素应为空格, got %q", row[0].ch)
	}
	if row[len(row)-1].ch != '@' {
		t.Errorf("最亮像素应为 @, got %q", row[len(row)-1].ch)
	}
}

func TestResolveColor(t *testing.T) {
	for _, c := range []struct {
		mode string
		want bool
	}{
		{"always", true},
		{"never", false},
	} {
		got, err := resolveColor(c.mode)
		if err != nil || got != c.want {
			t.Errorf("resolveColor(%q) = %v, %v; want %v, nil", c.mode, got, err, c.want)
		}
	}
	if _, err := resolveColor("bad"); err == nil {
		t.Error("无效模式应返回错误")
	}
}
