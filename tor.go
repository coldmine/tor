package main

import (
	"fmt"
	"os"
	"time"
	term "github.com/nsf/termbox-go"
	"io/ioutil"
	"strings"
	"flag"
	"strconv"
	"unicode"
	"unicode/utf8"
	"github.com/mattn/go-runewidth"
)

// we use line, offset style. termbox use o, l style.
func SetCursor(l, o int) {
	term.SetCursor(o, l)
}

func SetCell(l, o int, r rune, fg, bg term.Attribute) {
	term.SetCell(o, l, r, fg, bg)
}

func SetTermboxCursor(c *Cursor, w *Window, ar *Area) {
	p := c.PositionInWindow(w)
	SetCursor(ar.min.l+p.l, ar.min.o+p.o)
}

func clearScreen(ar *Area) {
	for l := ar.min.l ; l < ar.max.l ; l++ {
		for o := ar.min.o ; o < ar.max.o ; o++ {
			SetCell(l, o, ' ', term.ColorDefault, term.ColorDefault)
		}
	}
}

// draw text inside of window at mainviewer
func drawScreen(ar *Area, w *Window, t *Text, sel *Selection, c *Cursor, mode string, moveMode bool) {
	for l , ln := range t.lines {
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
					eoc += taboffset
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
					eoc -= taboffset
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
			if sel.on && sel.Contains(Point{l,o}) {
				bg = term.ColorGreen
			}
			if l == c.l {
				if mode != "normal" || moveMode {
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
				for i:=0 ; i<taboffset ; i++ {
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
	for off:=0 ; off<termw ; off++ {
		SetCell(statusLine, off, ' ', term.ColorBlack, term.ColorWhite)
	}
	for off, r := range status {
		SetCell(statusLine, off, r, term.ColorBlack, term.ColorWhite)
	}
}

func parseEvent(ev term.Event, sel *Selection, moveMode *bool) []*Action {
	if ev.Type != term.EventKey {
		panic(fmt.Sprintln("what the..", ev.Type, "event?"))
	}

	switch ev.Key {
	case term.KeyCtrlW:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"exit"}}
	case term.KeyCtrlS:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"save"}}
	// move
	case term.KeyArrowLeft:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"left"}}
	case term.KeyArrowRight:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"right"}}
	case term.KeyArrowUp:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"up"}}
	case term.KeyArrowDown:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"down"}}
	// insert
	case term.KeyEnter:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"insert", value:"\n"}, &Action{kind:"insert", value:"autoIndent"}}
	case term.KeyCtrlN:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"eol"}, &Action{kind:"insert", value:"\n"}}
	case term.KeySpace:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"insert", value:" "}}
	case term.KeyTab:
		return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"insert", value:"\t"}}
	case term.KeyCtrlU:
		return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"removeTab"}}
	case term.KeyCtrlO:
		return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"insertTab"}}
	// delete : value will added after actual deletion.
	case term.KeyDelete:
		if sel.on {
			return []*Action{&Action{kind:"deleteSelection"}, &Action{kind:"selection", value:"off"}}
		} else {
			return []*Action{&Action{kind:"delete"}}
		}
	case term.KeyBackspace, term.KeyBackspace2:
		if sel.on {
			return []*Action{&Action{kind:"deleteSelection"}, &Action{kind:"selection", value:"off"}}
		} else {
			return []*Action{&Action{kind:"backspace"}}
		}
	// undo, redo
	case term.KeyCtrlZ:
		return []*Action{&Action{kind:"undo"}}
	case term.KeyCtrlY:
		return []*Action{&Action{kind:"redo"}}
	// copy, paste, cut
	case term.KeyCtrlC:
		return []*Action{&Action{kind:"copy"}, &Action{kind:"selection", value:"off"}}
	case term.KeyCtrlV:
		if sel.on {
			return []*Action{&Action{kind:"deleteSelection"}, &Action{kind:"selection", value:"off"}, &Action{kind:"paste"}}
		}
		return []*Action{&Action{kind:"paste"}}
	case term.KeyCtrlX:
		return []*Action{&Action{kind:"copy"}, &Action{kind:"deleteSelection"}, &Action{kind:"selection", value:"off"}}
	// find
	case term.KeyCtrlD:
		return []*Action{&Action{kind:"saveFindWord"}, &Action{kind:"modeChange", value:"find"}}
	case term.KeyCtrlF:
		return []*Action{&Action{kind:"modeChange", value:"find"}}
	case term.KeyCtrlG:
		return []*Action{&Action{kind:"modeChange", value:"gotoline"}}
	case term.KeyCtrlJ:
		return []*Action{&Action{kind:"moveMode"}}
	case term.KeyCtrlL:
		return []*Action{&Action{kind:"selectLine"}}
	default:
		if ev.Ch == 0 {
			return []*Action{&Action{kind:"none"}}
		}
		if (*moveMode) || (ev.Mod & term.ModAlt != 0) {
			switch ev.Ch {
			case 'j':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"left"}}
			case 'J':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"left"}}
			case 'l':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"right"}}
			case 'L':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"right"}}
			case 'i', 'q':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"up"}}
			case 'I', 'Q':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"up"}}
			case 'k', 'a':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"down"}}
			case 'K', 'A':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"down"}}
			case 'm':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"prevBowEow"}}
			case 'M':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"prevBowEow"}}
			case '.':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"nextBowEow"}}
			case '>':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"nextBowEow"}}
			case 'u':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"bocBolAdvance"}}
			case 'U':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"bocBolAdvance"}}
			case 'o':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"eolAdvance"}}
			case 'O':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"eolAdvance"}}
			case 'w':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"pageup"}}
			case 'W':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"pageup"}}
			case 's':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"pagedown"}}
			case 'S':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"pagedown"}}
			case 'e':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"bof"}}
			case 'E':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"bof"}}
			case 'd':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"eof"}}
			case 'D':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"eof"}}
			case 'n':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"nextGlobal"}}
			case 'N':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"nextGlobal"}}
			case 'h':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"prevGlobal"}}
			case 'H':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"prevGlobal"}}
			case ']', 'x':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"nextArg"}}
			case '}', 'X':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"nextArg"}}
			case '[', 'z':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"prevArg"}}
			case '{', 'Z':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"prevArg"}}
			case 'f':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"nextFindWord"}}
			case 'F':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"nextFindWord"}}
			case 'b':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"prevFindWord"}}
			case 'B':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"prevFindWord"}}
			case 'c':
				return []*Action{&Action{kind:"selection", value:"off"}, &Action{kind:"move", value:"matchingBracket"}}
			case 'C':
				return []*Action{&Action{kind:"selection", value:"on"}, &Action{kind:"move", value:"matchingBracket"}}
			default:
				return []*Action{&Action{kind:"none"}}
			}
		}
		if sel.on {
			return []*Action{&Action{kind:"deleteSelection"}, &Action{kind:"insert", value:string(ev.Ch)}}
		} else {
			return []*Action{&Action{kind:"insert", value:string(ev.Ch)}}
		}
	}
}

func do(a *Action, c *Cursor, sel *Selection, history *History, findStr *string, status *string, holdStatus *bool) {
	defer func() {
		if sel.on {
			sel.SetEnd(c)
		}
	}()
	switch a.kind {
	case "none":
		return
	case "selection":
		if a.value == "on" && !sel.on {
			sel.on = true
			sel.SetStart(c)
		} else if a.value == "off" {
			sel.on = false
		}
	case "move":
		switch a.value {
		case "left":
			c.MoveLeft()
		case "right":
			c.MoveRight()
		case "up":
			c.MoveUp()
		case "down":
			c.MoveDown()
		case "prevBowEow":
			c.MovePrevBowEow()
		case "nextBowEow":
			c.MoveNextBowEow()
		case "bol":
			c.MoveBol()
		case "eol":
			c.MoveEol()
		case "bocBolAdvance":
			c.MoveBocBolAdvance()
		case "eolAdvance":
			c.MoveEolAdvance()
		case "pageup":
			c.PageUp()
		case "pagedown":
			c.PageDown()
		case "bof":
			c.MoveBof()
		case "eof":
			c.MoveEof()
		case "nextGlobal":
			c.GotoNextGlobalLineWithout(" \t#/{}()")
		case "prevGlobal":
			c.GotoPrevGlobalLineWithout(" \t#/{}()")
		case "nextArg":
			c.GotoNextAny("{(,)}")
			r, _ := c.RuneAfter()
			if r == '(' || r == '{'  {
				c.MoveRight()
			}
		case "prevArg":
			r, _ := c.RuneBefore()
			if r == '(' || r == '{'  {
				c.MoveLeft()
			}
			c.GotoPrevAny("{(,)}")
			r, _ = c.RuneAfter()
			if r == '(' || r == '{' {
				c.MoveRight()
			}
		case "nextFindWord":
			if *findStr != "" {
				c.GotoNext(*findStr)
			}
		case "prevFindWord":
			if *findStr != "" {
				c.GotoPrev(*findStr)
			}
		case "nextCursorWord":
			w :=c.Word()
			if w != "" {
				c.GotoNext(w)
			}
		case "prevCursorWord":
			w := c.Word()
			if w != "" {
				c.GotoPrev(w)
			}
		case "matchingBracket":
			c.GotoMatchingBracket()
		default:
			panic(fmt.Sprintln("what the..", a.value, "move?"))
		}
	case "saveFindWord":
		*findStr = c.Word()
		*status = fmt.Sprintf("find string : %v", *findStr)
		*holdStatus = true
	case "insert":
		if a.value == "autoIndent" {
			prevline := c.t.lines[c.l-1].data
			trimed := strings.TrimLeft(prevline, " \t")
			indent := prevline[:len(prevline)-len(trimed)]
			c.Insert(indent)
			a.value = indent
			return
		}
		c.Insert(a.value)
	case "delete":
		a.value = c.Delete()
	case "insertTab":
		var tabed []int
		if sel.on {
			tabed = c.Tab(sel)
			for _, l := range tabed {
				if l == sel.start.l {
					sel.start.o += taboffset
				}
			}
		} else {
			tabed = c.Tab(nil)
		}
		tabedStr := ""
		for _, l := range tabed {
			if tabedStr != "" {
				tabedStr += ","
			}
			tabedStr += strconv.Itoa(l)
		}
		a.value = tabedStr
	case "removeTab":
		var untabed []int
		if sel.on {
			untabed = c.UnTab(sel)
			for _, l := range untabed {
				if l == sel.start.l {
					sel.start.o -= taboffset
				}
			}
		} else {
			untabed = c.UnTab(nil)
		}
		untabedStr := ""
		for _, l := range untabed {
			if untabedStr != "" {
				untabedStr += ","
			}
			untabedStr += strconv.Itoa(l)
		}
		a.value = untabedStr
	case "backspace":
		a.value = c.Backspace()
	case "deleteSelection":
		if sel.on {
			a.value = c.DeleteSelection(sel)
			sel.on = false
		} else {
			a.value = c.Delete()
		}
	case "selectLine":
		c.MoveBol()
		if !sel.on {
			sel.on = true
			sel.SetStart(c)
		}
		c.MoveDown()
		sel.SetEnd(c)
	case "undo":
		if history.head == 0 {
			return
		}
		history.head--
		action := history.At(history.head)
		// status = fmt.Sprintf("undo : %v", action)
		// holdStatus = true
		switch action.kind {
		case "insert":
			c.Copy(action.afterCursor)
			for range action.value {
				c.Backspace()
			}
		case "insertTab":
			lines := strings.Split(action.value, ",")
			for _, lStr := range lines {
				if lStr == "" {
					continue
				}
				l, err := strconv.Atoi(lStr)
				if err != nil {
					panic(err)
				}
				err = c.t.lines[l].RemoveTab()
				if err != nil {
					panic(err)
				}
			}
			c.Copy(action.beforeCursor)
		case "paste":
			c.Copy(action.beforeCursor)
			for range action.value {
				c.Delete()
			}
		case "backspace":
			c.Copy(action.afterCursor)
			c.Insert(action.value)
		case "delete", "deleteSelection":
			c.Copy(action.afterCursor)
			c.Insert(action.value)
		case "removeTab":
			lines := strings.Split(action.value, ",")
			for _, lStr := range lines {
				if lStr == "" {
					continue
				}
				l, err := strconv.Atoi(lStr)
				if err != nil {
					panic(err)
				}
				c.t.lines[l].InsertTab()
			}
			c.Copy(action.beforeCursor)
		default:
			panic(fmt.Sprintln("what the..", action.kind, "history?"))
		}
	case "redo":
		if history.head == history.Len() {
			return
		}
		action := history.At(history.head)
		// status = fmt.Sprintf("redo : %v", action)
		// holdStatus = true
		history.head++
		switch action.kind {
		case "insert":
			c.Copy(action.beforeCursor)
			c.Insert(action.value)
		case "insertTab":
			lines := strings.Split(action.value, ",")
			for _, lStr := range lines {
				if lStr == "" {
					continue
				}
				l, err := strconv.Atoi(lStr)
				if err != nil {
					panic(err)
				}
				c.t.lines[l].InsertTab()
			}
			c.Copy(action.afterCursor)
		case "paste":
			c.Copy(action.beforeCursor)
			c.Insert(action.value)
		case "backspace":
			c.Copy(action.beforeCursor)
			for range action.value {
				c.Backspace()
			}
		case "delete", "deleteSelection":
			c.Copy(action.beforeCursor)
			for range action.value {
				c.Delete()
			}
		case "removeTab":
			lines := strings.Split(action.value, ",")
			for _, lStr := range lines {
				if lStr == "" {
					continue
				}
				l, err := strconv.Atoi(lStr)
				if err != nil {
					panic(err)
				}
				err = c.t.lines[l].RemoveTab()
				if err != nil {
					panic(err)
				}
			}
			c.Copy(action.afterCursor)
		default:
			panic(fmt.Sprintln("what the..", action.kind, "history?"))
		}
	default:
		panic(fmt.Sprintln("what the..", a.kind, "action?"))
	}
}


func main() {
	var f string
	if len(os.Args) == 1 {
		fmt.Println("please, set text file")
		os.Exit(1)
	} else {
		maybeFile := os.Args[len(os.Args)-1]
		if strings.HasPrefix(maybeFile, "-") || strings.ContainsAny(maybeFile, "=") {
			fmt.Println("please, set text file")
			os.Exit(1)
		} else {
			f = maybeFile
		}
	}
	var debug bool
	flag.BoolVar(&debug, "debug", false, "tor will create .history file for debugging.")
	flag.Parse()

	text, err := open(f)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = term.Init()
	if err != nil {
		panic(err)
	}
	defer term.Close()
	term.SetInputMode(term.InputAlt)
	// term.SetOutputMode(term.Output256)
	term.Clear(term.ColorDefault, term.ColorDefault)
	term.Flush()


	layout := NewLayout()
	mainview := layout.MainViewerBound()
	win := NewWindow(mainview.Size())
	// drawbuf := textToDrawBuffer(text, selection)
	cursor := NewCursor(text)
	selection := NewSelection()
	history := newHistory()
	SetCursor(mainview.min.l, mainview.min.o)

	mode := "normal"
	moveMode := false

	edited := false
	status := ""
	holdStatus := false
	lastActStr := ""
	oldFindStr := ""
	findStr := ""
	findDirection := ""
	findJustStart := false
	copied := ""
	gotolineStr := ""

	events := make(chan term.Event, 20)
	go func() {
		for {
			events <- term.PollEvent()
		}
	}()
	for {
		win.Follow(cursor, 3)
		clearScreen(mainview)
		drawScreen(mainview, win, text, selection, cursor, mode, moveMode)

		if mode == "exit" {
			status = fmt.Sprintf("Buffer modified. Do you really want to quit? (y/n)")
		} else if mode == "gotoline" {
			status = fmt.Sprintf("goto : %v", gotolineStr)
		} else if mode == "find" {
			status = fmt.Sprintf("find(%v) : %v", findDirection, findStr)
		} else {
			moveModeStr := ""
			if moveMode {
				moveModeStr = "(move mode)"
			}
			if !holdStatus {
				if selection.on {
					status = fmt.Sprintf("%v %v    selection on : (%v, %v) - (%v, %v)", f, moveModeStr, selection.start.l+1, selection.start.o, selection.end.l+1, selection.end.o)
				} else {
					status = fmt.Sprintf("%v %v    linenum:%v, byteoff:%v, visoff:%v, cursoroff:%v", f, moveModeStr, cursor.l+1, cursor.b, cursor.v, cursor.o)
				}
			}
		}
		printStatus(status)
		holdStatus = false

		SetTermboxCursor(cursor, win, mainview)
		term.Flush()

		// wait for keyboard input
		select {
		case ev := <-events:
			switch ev.Type {
			case term.EventKey:
				if mode == "exit" {
					if ev.Ch == 'y' {
						return
					} else if ev.Ch == 'n' || ev.Key == term.KeyEsc || ev.Key == term.KeyCtrlK {
						mode = "normal"
						term.SetInputMode(term.InputAlt)
					}
					continue
				} else if mode == "gotoline" {
					if ev.Key == term.KeyEsc || ev.Key == term.KeyCtrlK {
						gotolineStr = ""
						mode = "normal"
						term.SetInputMode(term.InputAlt)
						continue
					} else if ev.Key == term.KeyEnter {
						l, err := strconv.Atoi(gotolineStr)
						if err == nil {
							if l != 0 {
								l-- // internal line number and showing line number are different.
							}
							cursor.GotoLine(l)
						}
						gotolineStr = ""
						mode = "normal"
						term.SetInputMode(term.InputAlt)
						continue
					} else if ev.Key == term.KeyBackspace || ev.Key == term.KeyBackspace2 {
						if gotolineStr == "" {
							continue
						}
						_, rlen := utf8.DecodeLastRuneInString(gotolineStr)
						gotolineStr = gotolineStr[:len(gotolineStr)-rlen]
						continue
					} else if ev.Ch != 0 {
						_, err := strconv.Atoi(string(ev.Ch))
						if err == nil {
							gotolineStr += string(ev.Ch)
						}
						continue
					}
					continue
				} else if mode == "find" {
					if ev.Key == term.KeyEsc || ev.Key == term.KeyCtrlK {
						findStr = oldFindStr
						mode = "normal"
						term.SetInputMode(term.InputAlt)
					} else if ev.Key == term.KeyCtrlF {
						if findDirection != "next" && findDirection != "prev" {
							panic("what kind of find direction?")
						}
						if findDirection == "next" {
							findDirection = "prev"
						} else {
							findDirection = "next"
						}
					} else if ev.Key == term.KeyBackspace || ev.Key == term.KeyBackspace2 {
						if findJustStart {
							findStr = ""
							continue
						}
						_, rlen := utf8.DecodeLastRuneInString(findStr)
						findStr = findStr[:len(findStr)-rlen]
					} else if ev.Key == term.KeySpace {
						findStr += " "
					} else if ev.Ch != 0 {
						if findJustStart {
							findJustStart = false
							findStr = ""
						}
						findStr += string(ev.Ch)
					} else if ev.Key == term.KeyCtrlJ {
						findDirection = "prev"
						if findStr == "" {
							continue
						}
						cursor.GotoPrev(findStr)
						oldFindStr = findStr // so next time we can run find mode with current findStr.
					} else if ev.Key == term.KeyCtrlL {
						findDirection = "next"
						if findStr == "" {
							continue
						}
						cursor.GotoNext(findStr)
						oldFindStr = findStr // so next time we can run find mode with current findStr.
					} else if ev.Key == term.KeyCtrlI {
						if findStr == "" {
							continue
						}
						cursor.GotoFirst(findStr)
					} else if ev.Key == term.KeyCtrlK {
						if findStr == "" {
							continue
						}
						cursor.GotoLast(findStr)
					} else if ev.Key == term.KeyCtrlU {
						if findStr == "" {
							continue
						}
						cursor.GotoPrevWord(findStr)
					} else if ev.Key == term.KeyCtrlO {
						if findStr == "" {
							continue
						}
						cursor.GotoNextWord(findStr)
					} else if ev.Key == term.KeyEnter {
						if findStr == "" {
							continue
						}
						if findDirection == "next"{
							cursor.GotoNext(findStr)
						} else if findDirection == "prev" {
							cursor.GotoPrev(findStr)
						}
						oldFindStr = findStr // so next time we can run find mode with current findStr.
					}
					continue
				}

				if moveMode {
					if ev.Key == term.KeyCtrlJ || ev.Key == term.KeyCtrlK {
						moveMode = false
						continue
					}
				}

				actions := parseEvent(ev, selection, &moveMode)
				for _, a := range actions {
					if a.kind == "modeChange" {
						if a.value == "find" {
							if selection.on {
								min, max := selection.MinMax()
								findStr = text.DataInside(min, max)
							}
							findDirection = "next"
							findJustStart = true
						}
						mode = a.value
						moveMode = false
						term.SetInputMode(term.InputEsc)
						continue
					} else if a.kind == "moveMode" {
						moveMode = true
						continue
					}

					beforeCursor := *cursor

					if a.kind == "exit" {
						if !edited {
							return
						}
						mode = "exit"
						term.SetInputMode(term.InputEsc)
						continue
					} else if a.kind == "save" {
						err := save(f, text)
						if err != nil {
							panic(err)
						}
						edited = false
						status = fmt.Sprintf("successfully saved : %v", f)
						holdStatus = true
					} else if a.kind == "copy" {
						if selection.on {
							minc, maxc := selection.MinMax()
							copied = text.DataInside(minc, maxc)
						} else {
							r, _ := cursor.RuneAfter()
							copied = string(r)
						}
					} else if a.kind == "paste" {
						cursor.Insert(copied)
						a.value = copied
					} else {
						do(a, cursor, selection, history, &findStr, &status, &holdStatus)
					}
					switch a.kind {
					case "insert", "delete", "backspace", "deleteSelection", "paste", "insertTab", "removeTab":
						// remember the action.
						edited = true
						nc := history.Cut(history.head)
						if nc != 0 {
							lastActStr = ""
						}
						if a.kind == "insert" || a.kind == "delete" || a.kind == "backspace" {
							if a.kind == lastActStr {
								lastAct, err := history.Pop()
								if err != nil {
									panic(err)
								}
								history.head--
								beforeCursor = lastAct.beforeCursor
								if a.kind == "insert" || a.kind == "delete" {
									a.value = lastAct.value + a.value
								} else if a.kind == "backspace" {
									a.value = a.value + lastAct.value
								}
							}
						}
						a.beforeCursor = beforeCursor
						if a.kind == "deleteSelection" {
							a.beforeCursor, _ = selection.MinMax();
						}
						a.afterCursor = *cursor
						history.Add(a)
						history.head++
					}
					lastActStr = a.kind
					lastAct := history.Last()
					if debug && lastAct != nil {
						historyFileString := ""
						for i, a := range history.actions {
							if i != 0 {
								historyFileString += "\n"
							}
							historyFileString += fmt.Sprintf("%v, %v", a, history.head)
						}
						ioutil.WriteFile(extendFileName(f, ".history"), []byte(historyFileString), 0755)
					}
				}
			case term.EventResize:
				min := mainview.min
				o, l := term.Size()
				mainview = &Area{min, Point{min.l+l, min.o+o}}
				win.Resize(mainview.Size())
			}
		case <-time.After(time.Second):
			holdStatus = true
		}
	}
}
