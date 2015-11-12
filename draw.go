package main

import (
	"github.com/mattn/go-runewidth"
	term "github.com/nsf/termbox-go"
	"strconv"
	"unicode"
	"unicode/utf8"
)

func SetCell(l, o int, r rune, fg, bg term.Attribute) {
	term.SetCell(o, l, r, fg, bg)
}

func clearScreen(ar *Area) {
	for l := ar.min.l; l < ar.max.l; l++ {
		for o := ar.min.o; o < ar.max.o; o++ {
			SetCell(l, o, ' ', term.ColorDefault, term.ColorDefault)
		}
	}
}

func resizeScreen(ar *Area, w *Window) {
	min := ar.min
	o, l := term.Size()
	*ar = Area{min, Point{min.l + l, min.o + o}}
	w.Resize(ar.Size())
}

// draw text inside of window at mainarea.
func drawScreen(ar *Area, w *Window, t *Text, sel *Selection, c *Cursor, mode string) {
	for l, ln := range t.lines {
		if l < w.min.l || l >= w.max.l {
			continue
		}

		inStr := false
		inStrStarter := ' '
		inStrFinished := false
		commented := false
		oldR := ' '
		oldOldR := ' '
		var oldBg term.Attribute

		eoc := 0
		if ln.data != "" {
			// ++
			for _, r := range ln.data {
				if r == '\t' {
					eoc += t.tabWidth
				} else {
					eoc += runewidth.RuneWidth(r)
				}
			}
			// --
			remain := ln.data
			for {
				if remain == "" {
					break
				}
				r, rlen := utf8.DecodeLastRuneInString(remain)
				remain = remain[:len(remain)-rlen]
				if !unicode.IsSpace(r) {
					break
				}
				if r == '\t' {
					eoc -= t.tabWidth
				} else {
					eoc -= runewidth.RuneWidth(r)
				}
			}
		}

		// draw
		o := 0
		for _, r := range ln.data {
			if o >= w.max.o {
				break
			}

			bg := term.ColorDefault
			if o >= eoc {
				bg = term.ColorYellow
			}
			if sel.on && sel.Contains(Point{l, o}) {
				bg = term.ColorGreen
			}
			if l == c.l {
				if mode == "find" || mode == "gotoline" {
					bg = term.ColorCyan
				}
			}
			if r == '/' && oldR == '/' && oldOldR != '\\' {
				commented = true
				SetCell(l-w.min.l+ar.min.l, o-w.min.o+ar.min.o-1, '/', term.ColorMagenta, oldBg) // hacky way to color the first '/' cell.
			}
			if inStrFinished {
				inStr = false
				inStrStarter = ' '
			}
			if r == '\'' || r == '"' {
				if !(oldR == '\\' && oldOldR != '\\') {
					if !inStr {
						inStr = true
						inStrStarter = r
						inStrFinished = false
					} else if inStrStarter == r {
						inStrFinished = true
					}
				}
			}

			fg := term.ColorWhite
			if commented {
				fg = term.ColorMagenta
			} else if inStr {
				if inStrStarter == '\'' {
					fg = term.ColorYellow
				} else {
					fg = term.ColorRed
				}
			} else {
				_, err := strconv.Atoi(string(r))
				if err == nil {
					fg = term.ColorCyan
				}
			}

			if r == '\t' {
				for i := 0; i < t.tabWidth; i++ {
					if o >= w.min.o {
						SetCell(l-w.min.l+ar.min.l, o-w.min.o+ar.min.o, rune(' '), fg, bg)
					}
					o += 1
				}
			} else {
				if o >= w.min.o {
					SetCell(l-w.min.l+ar.min.l, o-w.min.o+ar.min.o, rune(r), fg, bg)
				}
				o += runewidth.RuneWidth(r)
			}

			oldOldR = oldR
			oldR = r
			oldBg = bg
		}
	}
}

func printStatus(status string) {
	termw, termh := term.Size()
	statusLine := termh - 1
	// clear
	for i := 0; i < termw; i++ {
		SetCell(statusLine, i, ' ', term.ColorBlack, term.ColorWhite)
	}
	// draw
	o := 0
	for _, r := range status {
		SetCell(statusLine, o, r, term.ColorBlack, term.ColorWhite)
		o += runewidth.RuneWidth(r)
	}
}
