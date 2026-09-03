//go:build !windows

package main

import "os"

// 非 Windows 平台的兜底实现：不探测终端尺寸，输出一律按非终端处理。
func terminalSize(f *os.File) (w, h int, ok bool) { return 0, 0, false }

func isTerminal(f *os.File) bool { return false }
