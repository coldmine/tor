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
)

// we use line, offset style. termbox use o, l style.
func SetCursor(l, o int) {
       term.SetCursor(o, l)
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println("please, set text file")
		os.Exit(1)
	}

	f := os.Args[len(os.Args)-1]
	if strings.HasPrefix(f, "-") || strings.ContainsAny(f, "=") {
		fmt.Println("please, set text file")
		os.Exit(1)
	}

	finfo := strings.Split(f, ":")
	f = finfo[0]
	initLine := -1
	initOff := -1
	switch len(finfo) {
	case 1:
	case 2:
		l, err := strconv.Atoi(finfo[1])
		if err != nil {
			fmt.Println("parse file argument error: cannot convert line num to int")
			os.Exit(1)
		}
		initLine = l
	case 3:
		l, err := strconv.Atoi(finfo[1])
		if err != nil {
			fmt.Println("parse file argument error: cannot convert line num to int")
			os.Exit(1)
		}
		o, err := strconv.Atoi(finfo[2])
		if err != nil {
			fmt.Println("parse file argument error: cannot convert line offset to int")
			os.Exit(1)
		}
		initLine, initOff = l, o
	default:
		fmt.Println("parse file argument error: too many colons")
		os.Exit(1)
	}


	var new bool
	flag.BoolVar(&new, "new", false, "let tor to edit a new file.")
	var debug bool
	flag.BoolVar(&debug, "debug", false, "tor will create .history file for debugging.")
	flag.Parse()

	exist := true
	if _, err := os.Stat(f); os.IsNotExist(err) {
		exist = false
	}
	if !exist && !new {
		fmt.Println("file not exist. please retry with -new flag.")
		os.Exit(1)
	} else if exist && new {
		fmt.Println("file already exist.")
		os.Exit(1)
	}

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
	mainarea := layout.MainViewerBound()
	win := NewWindow(mainarea.Size())
	// drawbuf := textToDrawBuffer(text, selection)
	cursor := NewCursor(text)
	if initLine != -1 {
		l := initLine
		if l != 0 {
			l--
		}
		cursor.GotoLine(l)
		if initOff != -1 {
			cursor.o = cursor.OFromV(initOff)
			cursor.v = cursor.o
		}
	} else {
		l, b := loadLastPosition(f)
		cursor.GotoLine(l)
		cursor.SetOffsetsMaybe(b)
	}
	findmode := &FindMode{}
	gotolinemode := &GotoLineMode{}
	selection := NewSelection()
	history := newHistory()

	mode := "normal"

	edited := false
	status := ""
	holdStatus := false
	lastActStr := ""
	copied := ""

	events := make(chan term.Event, 20)
	go func() {
		for {
			events <- term.PollEvent()
		}
	}()
	for {
		win.Follow(cursor, 3)
		clearScreen(mainarea)
		drawScreen(mainarea, win, text, selection, cursor, mode)

		if mode == "exit" {
			status = fmt.Sprintf("Buffer modified. Do you really want to quit? (y/n)")
		} else if mode == "gotoline" {
			status = fmt.Sprintf("goto : %v", gotolinemode.linestr)
		} else if mode == "find" {
			status = fmt.Sprintf("find : %v", findmode.findstr)
		} else {
			mm := ""
			if mode == "move" {
				mm = "(move mode)"
			}
			if !holdStatus {
				if selection.on {
					status = fmt.Sprintf("%v %v    selection on : (%v, %v) - (%v, %v)", f, mm, selection.start.l+1, selection.start.o, selection.end.l+1, selection.end.o)
				} else {
					status = fmt.Sprintf("%v %v    linenum:%v, byteoff:%v, visoff:%v, cursoroff:%v", f, mm, cursor.l+1, cursor.b, cursor.v, cursor.o)
				}
			}
		}
		printStatus(status)
		holdStatus = false

		winP := cursor.PositionInWindow(win)
		SetCursor(mainarea.min.l+winP.l, mainarea.min.o+winP.o)

		term.Flush()

		// wait for keyboard input
		select {
		case ev := <-events:
			switch ev.Type {
			case term.EventKey:
				if mode == "exit" {
					if ev.Ch == 'y' {
						saveLastPosition(f, cursor.l, cursor.b)
						return
					} else if ev.Ch == 'n' || ev.Key == term.KeyCtrlK {
						mode = "normal"
					}
					continue
				} else if mode == "gotoline" {
					gotolinemode.Handle(ev, cursor, &mode)
					continue
				} else if mode == "find" {
					findmode.Handle(ev, cursor, &mode)
					continue
				} else if mode == "move" {
					if ev.Key == term.KeyCtrlK {
						mode = "normal"
						continue
					}
				}

				actions := parseEvent(ev, selection, &mode)
				for _, a := range actions {
					if a.kind == "modeChange" {
						if a.value == "find" {
							if selection.on {
								min, max := selection.MinMax()
								findmode.findstr = text.DataInside(min, max)
								findmode.juststart = true
							}
						}
						mode = a.value
						continue
					}

					beforeCursor := *cursor

					if a.kind == "exit" {
						if !edited {
							saveLastPosition(f, cursor.l, cursor.b)
							return
						}
						mode = "exit"
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
						saveCopyString(copied)
					} else if a.kind == "paste" {
						if copied == "" {
							copied = loadCopyString()
						}
						cursor.Insert(copied)
						a.value = copied
					} else {
						do(a, cursor, selection, history, &status, &holdStatus)
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
				resizeScreen(mainarea, win)
			}
		case <-time.After(time.Second):
			holdStatus = true
			// It seems maximize the terminal
			// doesn't call term.EventResize.
			// So manually do it.
			resizeScreen(mainarea, win)
		}
	}
}
