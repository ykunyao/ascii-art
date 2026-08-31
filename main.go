// ascii-art 把图片渲染成终端 ASCII 字符画。
// 支持 PNG / JPEG / GIF，可选真彩色输出与 GIF 动画循环播放。仅依赖标准库。
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"runtime"
	"strings"
	"time"
)

const defaultRamp = " .:-=+*#%@"

// 终端字符的高度约为宽度的两倍，渲染时按此校正纵横比。
const charAspect = 0.5

type cell struct {
	ch       byte
	r, g, b uint8
}

func main() {
	width := flag.Int("width", 100, "输出宽度（字符数）")
	rampFlag := flag.String("ramp", defaultRamp, "明暗字符梯度，从暗到亮")
	invert := flag.Bool("invert", false, "反转明暗映射（浅色背景的终端适用）")
	colorMode := flag.String("color", "auto", "彩色输出模式: auto, always, never")
	play := flag.Bool("play", false, "循环播放 GIF 动画（Ctrl+C 退出）")
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	if *rampFlag == "" {
		*rampFlag = defaultRamp
	}
	ramp := []byte(*rampFlag)
	if *invert {
		for i, j := 0, len(ramp)-1; i < j; i, j = i+1, j-1 {
			ramp[i], ramp[j] = ramp[j], ramp[i]
		}
	}
	useColor, err := resolveColor(*colorMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		die(err)
	}
	defer f.Close()

	if *play {
		if err := playGIF(f, *width, ramp, useColor); err != nil {
			die(err)
		}
		return
	}

	img, _, err := image.Decode(f)
	if err != nil {
		die(err)
	}
	printCells(render(img, *width, ramp), useColor)
}

func usage() {
	fmt.Fprintf(os.Stderr, "ascii-art — 把图片渲染成终端 ASCII 字符画\n\n用法: ascii-art [选项] <图片路径>\n\n支持 PNG / JPEG / GIF；GIF 加 -play 可循环播放动画。\n")
	flag.PrintDefaults()
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "ascii-art:", err)
	os.Exit(1)
}

func resolveColor(mode string) (bool, error) {
	switch mode {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
		if runtime.GOOS == "windows" {
			return os.Getenv("WT_SESSION") != "" ||
				os.Getenv("ConEmuANSI") == "ON" ||
				os.Getenv("ANSICON") != "", nil
		}
		term := os.Getenv("TERM")
		return term != "" && term != "dumb", nil
	default:
		return false, fmt.Errorf("无效的 -color 值 %q（可选 auto, always, never）", mode)
	}
}

// render 将图片按 outW 个字符的宽度采样为 cell 网格。
func render(img image.Image, outW int, ramp []byte) [][]cell {
	if outW < 1 {
		outW = 80
	}
	b := img.Bounds()
	outH := max(1, int(float64(b.Dy())*(float64(outW)/float64(b.Dx()))*charAspect))
	rows := make([][]cell, outH)
	n := len(ramp)
	for y := 0; y < outH; y++ {
		row := make([]cell, outW)
		sy := b.Min.Y + (2*y+1)*b.Dy()/(2*outH)
		for x := 0; x < outW; x++ {
			sx := b.Min.X + (2*x+1)*b.Dx()/(2*outW)
			r16, g16, b16, _ := img.At(sx, sy).RGBA()
			lum := 0.2126*float64(r16) + 0.7152*float64(g16) + 0.0722*float64(b16)
			idx := int(lum*float64(n-1)/65535.0 + 0.5)
			row[x] = cell{ramp[idx], uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8)}
		}
		rows[y] = row
	}
	return rows
}

func printCells(cells [][]cell, useColor bool) {
	var sb strings.Builder
	for _, row := range cells {
		if useColor {
			for _, c := range row {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%c", c.r, c.g, c.b, c.ch)
			}
			sb.WriteString("\x1b[0m")
		} else {
			for _, c := range row {
				sb.WriteByte(c.ch)
			}
		}
		sb.WriteByte('\n')
	}
	fmt.Print(sb.String())
}

// playGIF 循环播放 GIF 各帧，光标归位复用已打印的行避免闪烁。
func playGIF(f *os.File, outW int, ramp []byte, useColor bool) error {
	g, err := gif.DecodeAll(f)
	if err != nil {
		return err
	}
	frames, err := composeFrames(g)
	if err != nil {
		return err
	}
	fmt.Print("\x1b[2J")
	for {
		for i, fr := range frames {
			fmt.Print("\x1b[H")
			printCells(render(fr, outW, ramp), useColor)
			d := 5
			if i < len(g.Delay) && g.Delay[i] >= 2 {
				d = g.Delay[i]
			}
			time.Sleep(time.Duration(d) * 10 * time.Millisecond)
		}
	}
}

// composeFrames 把各帧按顺序合成到画布上（按“不清除”处理帧间保留），
// 并逐帧快照，避免后续帧污染已记录的画面。
func composeFrames(g *gif.GIF) ([]image.Image, error) {
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("GIF 中没有帧")
	}
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	frames := make([]image.Image, 0, len(g.Image))
	for _, fr := range g.Image {
		draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		snap := image.NewRGBA(canvas.Rect)
		draw.Draw(snap, snap.Rect, canvas, canvas.Rect.Min, draw.Src)
		frames = append(frames, snap)
	}
	return frames, nil
}
