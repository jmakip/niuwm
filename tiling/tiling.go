package tiling

import (
	"fmt"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

//TileMode tiling mode TileHori, TileVert
type TileMode uint16

//TileHori side by side
//TileVert on top of each other
const (
	TileHori = 0
	TileVert = 1
	//TileStack = 2
)

//Area the drawing boundaries
type Area struct {
	X      uint32
	Y      uint32
	Width  uint32
	Height uint32
}

//Workspace contains root tile
type Workspace struct {
	Name   string
	Bounds Area
	Root   *Tile
}

//Tile contains mode area and actual X11 window
//if mode is vertical right is up and Left is bottom
//perhaps future add stacking where tiles go on top of each other...
type Tile struct {
	Mode   TileMode
	Bounds Area
	Wind   xproto.Window
	Right  *Tile
	Left   *Tile
}

//Insert boilerplate code for tile
func (ws *Workspace) Insert(m TileMode, focus xproto.Window, w xproto.Window) {
	if ws == nil {
		return
	}
	if ws.Root.HasBranch() == false {
		ws.Root.Right = &Tile{m, ws.Root.Bounds, w, nil, nil}
		return
	}
	ws.Root.Right.Insert(m, focus, w)
	ws.Root.Right.reallocBounds(ws.Bounds)
}
func (ws *Workspace) Delete(w xproto.Window) {
	if ws == nil {
		return
	}
	if ws.Root.HasBranch() == false {
		return
	}
	ws.Root.Delete(w)
	if ws.Root.Right != nil {
		ws.Root.Right.reallocBounds(ws.Bounds)
	}
}

func (ws *Workspace) Config(xConn *xgb.Conn, padX uint32, padY uint32) {
	if ws == nil {
		return
	}
	if ws.Root.HasBranch() == false {
		return
	}
	ws.Root.Config(xConn, padX, padY)
}

//returns status
func (t *Tile) HasBranch() bool {
	if t.Right == nil && t.Left == nil {
		return false
	}
	return true

}
func (t *Tile) insertNoFocus(m TileMode, w xproto.Window) {

	if w == 0 {
		return
	}
	if t == nil {
		return
	}
	var maxb Area
	var box Area
	maxb = t.Bounds
	//totally wrong place for this code
	if t.Wind == 0 && t.Right != nil {
		t = t.Right
	}
	if t.Wind != 0 { //not root window
		t.Boundaries(&maxb)

		//we could call function to count all branches with same tiling mode
		//and split by that amount
		if m == TileHori {
			box.Width = maxb.Width / 2
			box.Height = maxb.Height
			box.X = maxb.X + maxb.Width
			box.Y = maxb.Y
			t.Bounds.Width = box.Width
		}
		if m == TileVert {
			box.Width = maxb.Width
			box.Height = maxb.Height / 2
			box.X = maxb.X
			box.Y = maxb.Y + maxb.Height
			t.Bounds.Height = box.Height
		}
	} else {
		box = t.Bounds
	}
	ntile := Tile{Mode: m,
		Bounds: box,
		Wind:   w,
		Right:  nil,
		Left:   nil}
	if t.Right != nil && t.Right.Mode == ntile.Mode {
		ntile.Right = t.Right
		t.Right = &ntile
	} else if t.Left != nil && t.Left.Mode == ntile.Mode {
		ntile.Left = t.Left
		t.Left = &ntile

	} else if t.Right != nil && t.Left == nil {
		t.Left = &ntile
	} else {
		ntile.Right = t.Right
		ntile.Left = t.Left
		t.Left = nil
		t.Right = &ntile
	}
	if t.Wind == 0 {
		if t.Right != nil {
			t.Right.reallocBounds(maxb)
			return
		}
	}
	t.reallocBounds(maxb)

}

//Insert append window into tree determinated by focus
//if focus == nil inserts into root
func (t *Tile) Insert(m TileMode, focus xproto.Window, w xproto.Window) {
	if w == 0 {
		return
	}
	rt := t.Find(focus)
	if rt == nil {
		rt = t
	}
	rt.insertNoFocus(m, w)

}

//Find search given window from tile tree
//rt nil if not found
func (t *Tile) Find(w xproto.Window) (rt *Tile) {
	if t == nil {
		return nil
	}
	if w == t.Wind {
		rt = t
		return rt
	}
	rt = t.Right.Find(w)
	if rt != nil {
		return rt
	}
	rt = t.Left.Find(w)
	return rt
}

//FindWithParent search given window return tile containing it and its parent tile
//nil, nil if not found
//tile, nil if found but has no parent
func (t *Tile) FindWithParent(w xproto.Window) (rt *Tile, p *Tile) {
	if t == nil {
		return nil, nil
	}
	if w == t.Wind {
		return t, nil
	}
	if t.Right != nil {
		if w == t.Right.Wind {
			return t.Right, t
		}
	}
	if t.Left != nil {
		if w == t.Left.Wind {
			return t.Left, t
		}
	}
	rt, p = t.Right.FindWithParent(w)
	if rt != nil {
		return rt, p
	}
	rt, p = t.Left.FindWithParent(w)
	return rt, p
}

//Delete delete given window from tile tree
func (t *Tile) Delete(w xproto.Window) {
	//find with parent
	rt, p := t.FindWithParent(w)
	//If tile has no branches give area to parent.

	if rt.Right == nil && rt.Left == nil {
		if p == nil {
			return
		}
		if p.Mode == TileHori {
			if rt == p.Right {
				p.Bounds.Width += rt.Bounds.Width
				p.Right = nil
			}
			if rt == p.Left {
				p.Bounds.Width += rt.Bounds.Width
				p.Bounds.X = rt.Bounds.X
				p.Left = nil
			}
		}
		if p.Mode == TileVert {
			if rt == p.Right {
				p.Bounds.Height += rt.Bounds.Height
				p.Bounds.Y = rt.Bounds.Y
				p.Right = nil
			}
			if rt == p.Left {
				p.Bounds.Height += rt.Bounds.Height
				p.Left = nil
			}
		}
		maxb := p.Bounds
		p.Boundaries(&maxb)
		p.reallocBounds(maxb)
		return
	}
	//If tile has any branches give area to them
	maxb := rt.Bounds
	rt.Boundaries(&maxb)
	//semi easy cases first if only one branch realloc area to that and connect to parent
	if rt.Right == nil {
		la := rt.Left
		if p.Right == rt {
			p.Right = la
		}
		if p.Left == rt {
			p.Left = la
		}
		la.reallocBounds(maxb)
		return
	}
	if rt.Left == nil {
		ra := rt.Right
		if p.Right == rt {
			p.Right = ra
		}
		if p.Left == rt {
			p.Left = ra
		}
		ra.reallocBounds(maxb)
		return
	}
	//hard case. we will just connect orphaned branches together by searching
	//free branch from Right side and insert Left to that
	ra := rt.Right
	la := rt.Left
	ra.insertFirstEmpty(la)
	ra.reallocBounds(maxb)
	if p.Right == rt {
		p.Right = ra
		return
	}
	if p.Left == rt {
		p.Left = ra
		return
	}

}

//naively just inserts Left side
func (t *Tile) insertFirstEmpty(other *Tile) {
	if t.Left == nil {
		t.Left = other
		return
	}
	if t.Right == nil {
		t.Right = other
		return
	}
	t.Left.insertFirstEmpty(other)
	return
}

func (t *Tile) reallocBounds(b Area) {

	if t.Right != nil {
		var box Area
		if t.Right.Mode == TileVert {
			box.Height = b.Height / 2
			box.Width = b.Width
			box.X = b.X
			box.Y = b.Y + box.Height
			b.Height = box.Height
		} else {
			box.Height = b.Height
			box.Width = b.Width / 2
			box.X = b.X + box.Width
			box.Y = b.Y
			b.Width = box.Width
		}
		t.Right.reallocBounds(box)
	}
	if t.Left != nil {
		var box Area
		if t.Left.Mode == TileVert {
			box.Height = b.Height / 2
			box.Width = b.Width
			box.X = b.X
			box.Y = b.Y + box.Height
			b.Height = box.Height
		} else {
			box.Height = b.Height
			box.Width = b.Width / 2
			box.X = b.X + box.Width
			box.Y = b.Y
			b.Width = box.Width
		}
		t.Left.reallocBounds(box)
	}
	t.Bounds = b
}

//Bounds return maximum area
func (t *Tile) Boundaries(bounds *Area) {
	if t == nil {
		return
	}
	if bounds == nil { //replace with return?
		bounds = &Area{X: t.Bounds.X,
			Y:      t.Bounds.Y,
			Width:  t.Bounds.Width,
			Height: t.Bounds.Height}
	}
	if t.Bounds.X < bounds.X {
		bounds.X = t.Bounds.X
	}
	if t.Bounds.Y < bounds.Y {
		bounds.Y = t.Bounds.Y
	}
	if t.Bounds.X+t.Bounds.Width > bounds.X+bounds.Width {
		bounds.Width = bounds.X + bounds.Width - bounds.X
	}
	if t.Bounds.Y+t.Bounds.Height > bounds.Y+bounds.Height {
		bounds.Height = bounds.Y + bounds.Height - bounds.Y
	}
	t.Right.Boundaries(bounds)
	t.Left.Boundaries(bounds)
}

//Config map window to area
func (t *Tile) Config(xConn *xgb.Conn, padX uint32, padY uint32) {
	if t == nil {
		return
	}
	if t.Wind != 0 {
		err := xproto.ConfigureWindowChecked(xConn, t.Wind,
			xproto.ConfigWindowX|
				xproto.ConfigWindowY|
				xproto.ConfigWindowWidth|
				xproto.ConfigWindowHeight,
			[]uint32{t.Bounds.X + padX,
				t.Bounds.Y + padY,
				t.Bounds.Width - padX*2,
				t.Bounds.Height - padY*2}).Check()
		if err != nil {
			fmt.Println(err)
		}
	}
	if t.Right != nil {
		t.Right.Config(xConn, padX, padY)
	}
	if t.Left != nil {
		t.Left.Config(xConn, padX, padY)
	}
}
