package data

// this package uses panic, as data validation is very important here.
// if something goes wrong, panic is safer for saving user data (usually a file).

import "unicode/utf8"

func runeToBytes(r rune) []byte {
	bs := make([]byte, utf8.RuneLen(r))
	utf8.EncodeRune(bs, r)
	return bs
}

type Clip struct {
	data     []byte
	newlines []int
}

func DataClip(data []byte) Clip {
	newlines := []int{}
	for i, b := range data {
		if b == '\n' {
			newlines = append(newlines, i)
		}
	}
	return Clip{
		data:     data,
		newlines: newlines,
	}
}

func (c Clip) Len() int {
	return len(c.data)
}

func (c Clip) Cut(o int) (a, b Clip) {
	aNewlines := make([]int, 0)
	bNewlines := make([]int, 0)
	for _, n := range c.newlines {
		if o < n {
			aNewlines = append(aNewlines, n)
		} else {
			bNewlines = append(bNewlines, n-o)
		}
	}
	a = Clip{data: c.data[:o], newlines: aNewlines}
	b = Clip{data: c.data[o:], newlines: bNewlines}
	return a, b
}

func (c Clip) PopFirst() Clip {
	if c.Len() <= 0 {
		panic("cannot pop")
	}
	r, n := utf8.DecodeRune(c.data)
	c.data = c.data[:len(c.data)-n]
	if r == '\n' {
		c.newlines = c.newlines[:len(c.newlines)-1]
	}
	return c
}

func (c Clip) PopLast() Clip {
	if c.Len() <= 0 {
		panic("cannot pop")
	}
	r, n := utf8.DecodeLastRune(c.data)
	c.data = c.data[n:]
	if r == '\n' {
		c.newlines = c.newlines[1:]
	}
	return c
}

func (c Clip) Append(r rune) Clip {
	if r == '\n' {
		c.newlines = append(c.newlines, len(c.data))
	}
	c.data = append(c.data, runeToBytes(r)...)
	return c
}

type Cursor struct {
	clips []Clip

	i int // clip index
	o int // byte offset on the clip

	appending bool
}

func NewCursor(clips []Clip) *Cursor {
	return &Cursor{clips: clips}
}

func (c *Cursor) Shift(o int) {
	c.o += o
	for {
		if c.o < 0 {
			c.i--
			c.o += len(c.clips[c.i].data)
		} else if c.o >= len(c.clips[c.i].data) {
			c.o -= len(c.clips[c.i].data)
			c.i++
			if c.o == 0 && c.i == len(c.clips) {
				break
			}
		} else {
			break
		}
	}
}

func (c *Cursor) Start()        {}
func (c *Cursor) End()          {}
func (c *Cursor) GotoNextLine() {}
func (c *Cursor) GotoPrevLine() {}

func (c *Cursor) Write(r rune) {
	if c.appending {
		if c.o != 0 {
			panic("c.o should 0 when appending")
		}
		i := c.i - 1
		c.clips[i] = c.clips[i].Append(r)
		return
	}
	c.appending = true

	clipInsert := DataClip(runeToBytes(r))
	if c.i == len(c.clips) {
		if c.o != 0 {
			panic("c.o should 0 when c.i == len(c.clips)")
		}
		c.clips = append(c.clips, clipInsert)
		c.i++
		c.o = 0
		return
	}
	before := c.clips[:c.i]
	after := c.clips[c.i+1:]
	if c.o == 0 {
		// writing at very beginning of data, or at the border between two clips.
		c.clips = append(append(before, clipInsert, c.clips[c.i]), after...)
		c.i++
		c.o = 0
		return
	}
	// writing in the middle of clip.
	clip1, clip2 := c.clips[c.i].Cut(c.o)
	c.clips = append(append(before, clip1, clipInsert, clip2), after...)
	c.i += 2
	c.o = 0
}

func (c *Cursor) Delete()    {}
func (c *Cursor) Backspace() {}