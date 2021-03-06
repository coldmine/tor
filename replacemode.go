package main

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

type ReplaceMode struct {
	str   string
	start bool
	olds  []string
}

func (m *ReplaceMode) Start() {
	nm := tor.normal
	if nm.selection.on {
		m.str = nm.text.DataInside(nm.selection.MinMax())
	}
	m.start = true
}

func (m *ReplaceMode) End() {}

func (m *ReplaceMode) Handle(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlK:
		if len(m.olds) == 0 {
			m.str = ""
		} else {
			m.str = m.olds[len(m.olds)-1]
		}
		tor.ChangeMode(tor.normal)
	case tcell.KeyEnter:
		tor.ChangeMode(tor.normal)
		m.olds = append(m.olds, m.str)
		saveConfig("replace", m.str)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if m.start {
			m.str = ""
			m.start = false
			return
		}
		_, rlen := utf8.DecodeLastRuneInString(m.str)
		m.str = m.str[:len(m.str)-rlen]
	default:
		if ev.Modifiers()&tcell.ModAlt != 0 {
			return
		}
		if ev.Rune() != 0 {
			if m.start {
				m.str = ""
			}
			m.str += string(ev.Rune())
		}
		m.start = false
	}
}

func (m *ReplaceMode) Status() string {
	return fmt.Sprintf("replace : %v", m.str)
}

func (m *ReplaceMode) Error() string {
	return ""
}
