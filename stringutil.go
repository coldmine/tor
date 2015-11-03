package main

import (
	"github.com/mattn/go-runewidth"
	"unicode/utf8"
)

func vlen(s string) int {
	remain := s
	o := 0
	for len(remain) > 0 {
		r, rlen := utf8.DecodeRuneInString(remain)
		remain = remain[rlen:]
		if r == '\t' {
			o += taboffset
		} else {
			o += runewidth.RuneWidth(r)
		}
	}
	return o
}
