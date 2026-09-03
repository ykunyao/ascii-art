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
	cells := render(img, 8, []byte(defaultRamp), modeASCII, 0)
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

func TestRenderUniform(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	cells := render(img, 5, []byte(defaultRamp), modeASCII, 0)
	want := cells[0][0].ch
	for _, row := range cells {
		for _, c := range row {
			if c.ch != want {
				t.Fatalf("均色图应渲染为统一字符, got %q vs %q", c.ch, want)
			}
		}
	}
}

func TestRenderHalfMode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 2))
	for x := 0; x < 8; x++ {
		img.Set(x, 0, color.RGBA{255, 255, 255, 255})
		img.Set(x, 1, color.RGBA{0, 0, 0, 255})
	}
	cells := render(img, 8, []byte(defaultRamp), modeHalf, 0)
	if len(cells) != 1 {
		t.Fatalf("half 模式行数 = %d, want 1", len(cells))
	}
	c := cells[0][0]
	if c.r < 250 {
		t.Errorf("上半像素应为白, got %d", c.r)
	}
	if c.r2 != 0 {
		t.Errorf("下半像素应为黑, got %d", c.r2)
	}
}

func TestRenderMaxRows(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 4))
	cells := render(img, 8, []byte(defaultRamp), modeASCII, 1)
	if len(cells) != 1 {
		t.Fatalf("限高后行数 = %d, want 1", len(cells))
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
