// ascii-art renders images as ASCII art in the terminal.
// Supports PNG / JPEG / GIF, optional truecolor output, Unicode half-block
// mode, and looping GIF playback. Standard library only.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

const defaultRamp = " .:-=+*#%@"

// 终端字符的高度约为宽度的两倍，ASCII 模式按此校正纵横比；half 模式逐像素无需校正。
const charAspect = 0.5

const (
	modeASCII = iota
	modeHalf
)

type cell struct {
	ch         byte
	r, g, b    uint8 // ASCII 模式为字符颜色，half 模式为上半像素
	r2, g2, b2 uint8 // half 模式的下半像素
}

func main() {
	width := flag.Int("width", 0, "output width in characters; 0 = fit the terminal")
	rampFlag := flag.String("ramp", defaultRamp, "brightness ramp, dark to light (ASCII mode)")
	invert := flag.Bool("invert", false, "invert the brightness mapping (for light terminal backgrounds)")
	colorMode := flag.String("color", "auto", "color output: auto, always, never")
	modeFlag := flag.String("mode", "ascii", "render mode: ascii, half (colorized Unicode half-blocks, double vertical resolution)")
	play := flag.Bool("play", false, "loop a GIF animation (Ctrl+C to stop)")
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
	mode := modeASCII
	switch *modeFlag {
	case "ascii":
	case "half":
		mode = modeHalf
	default:
		fmt.Fprintf(os.Stderr, "invalid -mode value %q (ascii, half)\n", *modeFlag)
		os.Exit(2)
	}
	useColor, err := resolveColor(*colorMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if mode == modeHalf && !useColor {
		die(fmt.Errorf("half mode needs color output; use -color=auto or -color=always"))
	}

	in, err := openInput(flag.Arg(0))
	if err != nil {
		die(err)
	}
	data, err := io.ReadAll(in)
	in.Close()
	if err != nil {
		die(err)
	}

	// 终端自适应：输出被重定向（管道/文件）时 terminalSize 返回 false，保持默认行为
	outW := *width
	termW, termH, tty := terminalSize(os.Stdout)
	if outW <= 0 {
		if tty && termW > 8 {
			outW = termW - 1
		} else {
			outW = 100
		}
	} else if tty && outW > termW {
		outW = termW
	}
	maxRows := 0
	if tty && termH > 2 {
		maxRows = termH - 1
	}

	if *play {
		if err := playGIF(bytes.NewReader(data), outW, ramp, mode, useColor, maxRows); err != nil {
			die(err)
		}
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		die(err)
	}
	printCells(render(img, outW, ramp, mode, maxRows), mode, useColor)
}

func usage() {
	fmt.Fprintf(os.Stderr, `ascii-art — render images as ASCII art in the terminal

Usage: ascii-art [options] <image | URL | ->

Supports PNG / JPEG / GIF. "-" reads from stdin; http(s) URLs are downloaded.
Add -play to loop GIF animations in the terminal.
`)
	flag.PrintDefaults()
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "ascii-art:", err)
	os.Exit(1)
}

// openInput 支持本地路径、http(s) URL 和 "-"（stdin）。
func openInput(arg string) (io.ReadCloser, error) {
	if arg == "-" {
		return os.Stdin, nil
	}
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		resp, err := http.Get(arg)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to download %s: %s", arg, resp.Status)
		}
		return resp.Body, nil
	}
	return os.Open(arg)
}

func resolveColor(mode string) (bool, error) {
	switch mode {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
		if !isTerminal(os.Stdout) {
			return false, nil
		}
		if runtime.GOOS == "windows" {
			term := os.Getenv("TERM")
			return os.Getenv("WT_SESSION") != "" ||
				os.Getenv("ConEmuANSI") == "ON" ||
				os.Getenv("ANSICON") != "" ||
				(term != "" && term != "dumb"), nil
		}
		term := os.Getenv("TERM")
		return term != "" && term != "dumb", nil
	default:
		return false, fmt.Errorf("invalid -color value %q (auto, always, never)", mode)
	}
}

// render 将图片渲染为 printed 行 cell 网格。maxRows > 0 时超出则按比例缩小宽度。
func render(img image.Image, outW int, ramp []byte, mode int, maxRows int) [][]cell {
	if outW < 1 {
		outW = 80
	}
	src := toNRGBA(img)
	b := src.Rect
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil
	}
	factor := charAspect
	if mode == modeHalf {
		factor = 1
	}
	pixRows := int(float64(b.Dy())*(float64(outW)/float64(b.Dx()))*factor + 0.5)
	if mode == modeHalf {
		if pixRows%2 == 1 {
			pixRows--
		}
		if pixRows < 2 {
			pixRows = 2
		}
	}
	if pixRows < 1 {
		pixRows = 1
	}
	printed := pixRows
	if mode == modeHalf {
		printed = pixRows / 2
	}
	if maxRows > 0 && printed > maxRows {
		outW = max(1, outW*maxRows/printed)
		pixRows = int(float64(b.Dy())*(float64(outW)/float64(b.Dx()))*factor + 0.5)
		if mode == modeHalf {
			if pixRows%2 == 1 {
				pixRows--
			}
			if pixRows < 2 {
				pixRows = 2
			}
		}
		if pixRows < 1 {
			pixRows = 1
		}
	}
	grid := pixelGrid(src, outW, pixRows, ramp)
	if mode == modeASCII {
		return grid
	}
	// half 模式：相邻两行像素合并为一行，上像素作前景、下像素作背景
	out := make([][]cell, pixRows/2)
	for y := range out {
		row := make([]cell, outW)
		for x := 0; x < outW; x++ {
			top, bot := grid[2*y][x], grid[2*y+1][x]
			row[x] = cell{r: top.r, g: top.g, b: top.b, r2: bot.r, g2: bot.g, b2: bot.b}
		}
		out[y] = row
	}
	return out
}

// toNRGBA 把任意图像统一转为像素可直接索引的 NRGBA，避免采样时逐点走接口。
func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Rect, img, b.Min, draw.Src)
	return n
}

// pixelGrid 把源图划分为 outW×rows 的像素块网格，每块输出一个平均色 cell。
func pixelGrid(src *image.NRGBA, outW, rows int, ramp []byte) [][]cell {
	b := src.Rect
	n := len(ramp)
	grid := make([][]cell, rows)
	for y := 0; y < rows; y++ {
		row := make([]cell, outW)
		y0 := b.Min.Y + y*b.Dy()/rows
		y1 := b.Min.Y + (y+1)*b.Dy()/rows
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < outW; x++ {
			x0 := b.Min.X + x*b.Dx()/outW
			x1 := b.Min.X + (x+1)*b.Dx()/outW
			if x1 <= x0 {
				x1 = x0 + 1
			}
			r, g, bl := avgBox(src, x0, y0, x1, y1)
			lum := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)
			idx := int(lum*float64(n-1)/255.0 + 0.5)
			row[x] = cell{ch: ramp[idx], r: r, g: g, b: bl}
		}
		grid[y] = row
	}
	return grid
}

// avgBox 对 [x0,x1)×[y0,y1) 区域按预乘 alpha 求平均，返回合成到黑色背景上的 8bit RGB。
func avgBox(src *image.NRGBA, x0, y0, x1, y1 int) (r, g, b uint8) {
	var sr, sg, sb, sa, cnt uint32
	for y := y0; y < y1 && y < src.Rect.Max.Y; y++ {
		base := (y-src.Rect.Min.Y)*src.Stride + (x0-src.Rect.Min.X)*4
		for x := x0; x < x1 && x < src.Rect.Max.X; x++ {
			o := base + (x-x0)*4
			a := uint32(src.Pix[o+3])
			sr += uint32(src.Pix[o+0]) * a
			sg += uint32(src.Pix[o+1]) * a
			sb += uint32(src.Pix[o+2]) * a
			sa += a
			cnt++
		}
	}
	if cnt == 0 {
		return 0, 0, 0
	}
	ar, ag, ab, aa := sr/cnt, sg/cnt, sb/cnt, sa/cnt
	if aa == 0 {
		return 0, 0, 0
	}
	// ar 是 value*alpha 的均值（0..65025），除以平均 alpha 即还原直通色
	return uint8(ar / aa), uint8(ag / aa), uint8(ab / aa)
}

func printCells(cells [][]cell, mode int, useColor bool) {
	var sb strings.Builder
	for _, row := range cells {
		for _, c := range row {
			switch {
			case mode == modeHalf:
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", c.r, c.g, c.b, c.r2, c.g2, c.b2)
			case useColor:
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%c", c.r, c.g, c.b, c.ch)
			default:
				sb.WriteByte(c.ch)
			}
		}
		if mode == modeHalf || useColor {
			sb.WriteString("\x1b[0m")
		}
		sb.WriteByte('\n')
	}
	fmt.Print(sb.String())
}

// playGIF 循环播放 GIF 各帧，光标归位复用已打印的行避免闪烁；Ctrl+C 时清屏复原。
func playGIF(r io.Reader, outW int, ramp []byte, mode int, useColor bool, maxRows int) error {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return err
	}
	frames, err := composeFrames(g)
	if err != nil {
		return err
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	fmt.Print("\x1b[2J")
	for {
		for i, fr := range frames {
			fmt.Print("\x1b[H")
			printCells(render(fr, outW, ramp, mode, maxRows), mode, useColor)
			d := 5
			if i < len(g.Delay) && g.Delay[i] >= 2 {
				d = g.Delay[i]
			}
			select {
			case <-sig:
				fmt.Print("\x1b[0m\x1b[2J\x1b[H")
				return nil
			case <-time.After(time.Duration(d) * 10 * time.Millisecond):
			}
		}
	}
}

// composeFrames 把各帧按顺序合成到画布上（按“不清除”处理帧间保留），
// 并逐帧快照，避免后续帧污染已记录的画面。
func composeFrames(g *gif.GIF) ([]image.Image, error) {
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("GIF has no frames")
	}
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Max.X, b.Max.Y
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	frames := make([]image.Image, 0, len(g.Image))
	for _, fr := range g.Image {
		draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		snap := image.NewNRGBA(canvas.Rect)
		draw.Draw(snap, snap.Rect, canvas, canvas.Rect.Min, draw.Src)
		frames = append(frames, snap)
	}
	return frames, nil
}
