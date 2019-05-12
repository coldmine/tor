package main

import (
	"github.com/kybin/tor/cell"
	"github.com/mattn/go-runewidth"
	term "github.com/nsf/termbox-go"
)

func SetCell(l, o int, r rune, fg, bg term.Attribute) {
	term.SetCell(o, l, r, fg, bg)
}

// draw text inside of window at mainarea.
func drawScreen(norm *NormalMode, a *Area, c *Cursor) {
	w := a.Win
	// parse syntax
	if norm.dirty {
		norm.parser.ClearFrom(cell.Pt{L: w.Min().L, O: 0})
		norm.dirty = false
	}
	norm.parser.ParseTo(cell.Pt{L: w.Max().L + 1, O: 0})

	// draw
	for l, ln := range norm.text.lines {
		if l < w.Min().L || l >= w.Max().L {
			continue
		}
		origFg := term.ColorWhite
		origBg := term.ColorBlack
		o := 0
		for b, r := range ln.data {
			if o >= w.Max().O {
				break
			}

			fg := origFg
			bg := origBg
			for _, m := range norm.parser.Matches {
				if m.Range.Contains(cell.Pt{l, b}) {
					c := m.Color
					bg = c.Bg
					fg = c.Fg
					break
				}
			}

			if norm.selection.Contains(cell.Pt{l, b}) {
				bg = term.ColorGreen
			}
			if r == '\t' {
				for i := 0; i < norm.text.tabWidth; i++ {
					if o >= w.Min().O {
						SetCell(l-w.Min().L, o-w.Min().O+a.min.O, rune(' '), fg, bg)
					}
					o += 1
				}
			} else {
				if o >= w.Min().O {
					SetCell(l-w.Min().L, o-w.Min().O+a.min.O, rune(r), fg, bg)
				}
				o += runewidth.RuneWidth(r)
			}
		}
		// set original color to the last cell. (white and black)
		// if not set, the cursor's color will look different.
		SetCell(l-w.Min().L, o-w.Min().O+a.min.O, rune(' '), origFg, origBg)
	}

	if true {
		// show left edge on cursor line.
		// it does not draw 'on' the screen and should refine it. (move it some where?)
		SetCell(c.l-w.Min().L, a.min.O-1, rune('·'), term.ColorCyan, term.ColorBlack)
	}

}

// drawStatus draws current status of m at bottom of terminal.
// If m has Error, it will printed with red background.
func drawStatus(m Mode) {
	var bg term.Attribute
	var status string
	if m.Error() != "" {
		bg = term.ColorRed
		status = m.Error()
	} else {
		bg = term.ColorWhite
		status = m.Status()
	}

	termw, termh := term.Size()
	statusLine := termh - 1
	// clear and draw
	for i := 0; i < termw; i++ {
		SetCell(statusLine, i, ' ', term.ColorBlack, bg)
	}
	o := 0
	for _, r := range status {
		SetCell(statusLine, o, r, term.ColorBlack, bg)
		o += runewidth.RuneWidth(r)
	}
}
