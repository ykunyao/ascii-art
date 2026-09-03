//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = modkernel32.NewProc("GetConsoleScreenBufferInfo")
	procGetConsoleMode             = modkernel32.NewProc("GetConsoleMode")
)

type coord struct{ x, y int16 }

type smallRect struct{ left, top, right, bottom int16 }

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

// terminalSize 返回附加到 f 的控制台窗口尺寸（列数、行数）；f 不是控制台时返回 false。
func terminalSize(f *os.File) (w, h int, ok bool) {
	var info consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(f.Fd(), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, 0, false
	}
	return int(info.window.right-info.window.left) + 1,
		int(info.window.bottom-info.window.top) + 1, true
}

func isTerminal(f *os.File) bool {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	return r != 0
}
