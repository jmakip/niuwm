package main

import (
	"fmt"
	"log"
	"niuwm/tiling"
	"os/exec"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

var xconn *xgb.Conn
var setup *xproto.SetupInfo
var screen *xproto.ScreenInfo

/*
var atom_wm_protocols xproto.Atom
var atom_wm_deletewindow xproto.Atom
var atom_wm_take_focus xproto.Atom
*/
var eventTimestamp xproto.Timestamp
var windows []xproto.Window
var wmAtoms map[string]xproto.Atom
var cfg NiuCfg

var curTileMode tiling.TileMode
var curWs tiling.Workspace
var currFocus xproto.Window

//RemoveWindow deletes window from Workspace
func RemoveWindow(wind xproto.Window) {
	curWs.Delete(wind)
}

//AddWindow inserts new window as child of active window
func AddWindow(window xproto.Window) {
	//focus := focused_window()
	if curWs.Root.HasBranch() == false {
		curWs.Bounds = tiling.Area{X: 0, Y: 0, Width: uint32(screen.WidthInPixels), Height: uint32(screen.HeightInPixels)}
	}
	curWs.Insert(curTileMode, currFocus, window)
}

//getActiveWindow find current active window
func getActiveWindow() xproto.Window {
	atomname := "_NET_ACTIVE_WINDOW"

	fmt.Printf("%s\n", atomname)
	cookie := xproto.InternAtom(xconn, false, uint16(len(atomname)), atomname)
	reply, err := cookie.Reply()
	if err == nil {
		fmt.Printf("GOT COOKIE: \n")
		preply, err := xproto.GetPropertyUnchecked(xconn, false, screen.Root, reply.Atom, xproto.GetPropertyTypeAny, 0, 1).Reply()
		if err == nil {
			fmt.Printf("GOT reply: \n")
			if preply.ValueLen == 0 {
				fmt.Printf("GOT reply length zero: \n")
				return 0
			}
			if preply.Type != xproto.AtomWindow {
				fmt.Printf("GOT type != atomwindow: \n")
				return 0
			}
			var wind xproto.Window
			wind = xproto.Window(uint32(preply.Value[0]) +
				uint32(preply.Value[1])<<8 +
				uint32(preply.Value[2])<<16 +
				uint32(preply.Value[3])<<24)

			fmt.Printf("GOT ACTIVE WINDOW: %d\n", wind)
			return wind

		}
	}
	return 0
}

//focusedWindow get current window that is active
func focusedWindow() xproto.Window {
	xproto.GrabServer(xconn)
	defer xproto.UngrabServer(xconn)
	reply, err := xproto.GetInputFocus(xconn).Reply()
	if err != nil {
		fmt.Printf("could not get active window id\n")
		return 0
	}
	if reply.Focus == screen.Root {
		return 0
	}
	wind := reply.Focus
	for {
		reply, _ := xproto.QueryTree(xconn, wind).Reply()
		if reply == nil {
			return 0
		}
		if wind == reply.Root {
			break
		}
		if reply.Parent == reply.Root {
			break
		}
		wind = reply.Parent
	}

	return wind
}

//giveFocus give focus to window
func giveFocus(w xproto.Window) {
	if w == 0 {
		return
	}
	err := xproto.SetInputFocus(xconn, xproto.InputFocusPointerRoot,
		w, eventTimestamp).Check()
	if err != nil {
		fmt.Printf("could not set input focus\n")
	}

}

//moveFocusLeft move inside tree
func moveFocusLeft() {
	active := focusedWindow()

	if active == 0 {
		//no focused_window just give it to first
		if curWs.Root.Right != nil {
			giveFocus(curWs.Root.Right.Wind)
			return
		}
		if curWs.Root.Left != nil {
			giveFocus(curWs.Root.Left.Wind)
			return
		}
		return
	}
	tile, _ := curWs.Root.FindWithParent(active)
	if tile != nil {
		if tile.Left != nil {
			giveFocus(tile.Left.Wind)
			currFocus = active
			return
		}
	}
	if curWs.Root.Right != nil {
		giveFocus(curWs.Root.Right.Wind)
		currFocus = active
		return
	}

}

//moveFocusRight move right inside containers
func moveFocusRight() {
	active := focusedWindow()

	if active == 0 {
		//no focused_window just give it to first
		if curWs.Root.Right != nil {
			giveFocus(curWs.Root.Right.Wind)
			return
		}
		if curWs.Root.Left != nil {
			giveFocus(curWs.Root.Left.Wind)
			return
		}
		return
	}
	tile, _ := curWs.Root.FindWithParent(active)
	if tile != nil {
		if tile.Right != nil {
			giveFocus(tile.Right.Wind)
			currFocus = active
			return
		}
	}
	if curWs.Root.Right != nil {
		giveFocus(curWs.Root.Right.Wind)
		currFocus = active
		return
	}
}

//findWindow search window
func findWindow(w xproto.Window) (ret bool) {
	ret = false
	for _, window := range windows {
		if w == window {
			ret = true
			break
		}
	}
	return
}

//startApp Start application with parameters wait for it to finish
func startApp(name string, params string) {
	full := name + " " + params
	//TODO read shell env from config
	cmd := exec.Command("/bin/bash", "-c", full)
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Started %s waiting to finish\n", name)
	_ = cmd.Wait()

}

//getWMatoms pushes 3 atom requests to x server and then waits for them
func getWMatoms() {
	wmAtoms = make(map[string]xproto.Atom)
	atomNames := [3]string{"WM_PROTOCOLS", "WM_TAKE_FOCUS", "WM_DELETE_WINDOW"}
	var cookies [3]xproto.InternAtomCookie
	for i := 0; i < len(atomNames); i++ {
		cookies[i] = xproto.InternAtom(xconn, false, uint16(len(atomNames[i])),
			atomNames[i])
	}

	for i := 0; i < len(atomNames); i++ {
		reply, err := cookies[i].Reply()
		if err != nil {
			log.Fatal("err")
			panic(err)
		}
		if reply == nil {
			wmAtoms[atomNames[i]] = 0
			continue
		}
		wmAtoms[atomNames[i]] = reply.Atom
		fmt.Printf("success getting wm atoms")
	}

}

//handleButtonPress launch shortcut binds
func handleButtonPress(e xproto.ButtonPressEvent) {
	if e.State&xproto.ModMaskControl != 0 {
		go startApp("termite", "")
	}

	button := e.Detail
	if button == 9 {
		//close WM TODO proper now i just kill it
		panic("closing")
	}
	if button == 36 {
		go startApp("termite", "")
	}

}

//handleKeyPress launch shortcuts
func handleKeyPress(e xproto.KeyPressEvent) {
	keycode := byte(e.Detail)
	mod := e.State /*
		if e.State&xproto.ModMaskControl != 0 {
			if keycode == 36 {
				go start_app("termite", "")
			}
			if keycode == 9 {
				//close WM TODO proper now i just kill it
				panic("closing")
			}
		}*/
	for _, cmd := range cfg.Keybinds.Cmd {
		if mod == cmd.Mod && keycode == cmd.Keycode {
			currFocus = focusedWindow()
			go startApp(cmd.Cmd, cmd.Cmdparams)
			return
		}
	}
	for _, actions := range cfg.Keybinds.Actions {
		if mod == actions.Mod && keycode == actions.Keycode {
			if actions.Action == "logout" {
				panic("closing")
			}
			if actions.Action == "focus_left" {
				moveFocusLeft()
			}
			if actions.Action == "focus_right" {
				moveFocusRight()
			}
			if actions.Action == "tiling_mode_toggle" {
				if curTileMode == tiling.TileHori {
					curTileMode = tiling.TileVert
				} else {
					curTileMode = tiling.TileHori
				}

			}
		}
	}

}

//mapWindow show window on screen
func mapWindow(window xproto.Window) {
	if findWindow(window) != false {
		return
	}
	// Ensure that we can manage this window.
	if err := xproto.ConfigureWindowChecked(
		xconn,
		window,
		xproto.ConfigWindowBorderWidth,
		[]uint32{
			0,
		}).Check(); err != nil {
		return
	}

	// Get notifications when this window is deleted.
	if err := xproto.ChangeWindowAttributesChecked(
		xconn,
		window,
		xproto.CwEventMask,
		[]uint32{
			xproto.EventMaskStructureNotify |
				xproto.EventMaskEnterWindow}).Check(); err != nil {

		return
	}
	if window != screen.Root {
		//windows = append(windows, window)
		AddWindow(window)
	}
}

//unmapWindow remove window
func unmapWindow(window xproto.Window) {
	RemoveWindow(window)
}

//queryWindows search windows under root window and map them
func queryWindows() {
	//perhaps remove all from windows first
	windows = nil
	tree, err := xproto.QueryTree(xconn, screen.Root).Reply()
	if err != nil {
		panic(err)
	}
	if tree != nil {
		for i, wind := range tree.Children {
			if i > 0 {
				mapWindow(wind)
			}
		}
	}

}

//tileWindows organize windows into Workspace
func tileWindows() {
	var pad = uint32(10)
	var screenX = uint32(screen.WidthInPixels)
	var screenY = uint32(screen.HeightInPixels)
	var nroWindows = uint32(len(windows))
	var tileSizeX uint32
	if nroWindows == 0 {
		return
	}
	if nroWindows == 1 {
		tileSizeX = screenX - pad
	} else {
		tileSizeX = (screenX / (nroWindows)) - pad
	}
	for index, wind := range windows {
		var idx = uint32(index)

		x := pad + (pad+tileSizeX+pad)*uint32(idx)
		err := xproto.ConfigureWindowChecked(xconn, wind,
			xproto.ConfigWindowX|
				xproto.ConfigWindowY|
				xproto.ConfigWindowWidth|
				xproto.ConfigWindowHeight,
			[]uint32{x, 0, tileSizeX, screenY}).Check()
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Printf("NRO WINDOWS %d \n", len(windows))
}

//grabKeyEvents notify XCB that we want listen these keyevents
func grabKeyEvents() {
	for _, cmd := range cfg.Keybinds.Cmd {
		xproto.GrabKey(xconn, true, screen.Root, cmd.Mod,
			xproto.Keycode(cmd.Keycode),
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	}
	for _, action := range cfg.Keybinds.Actions {
		xproto.GrabKey(xconn, true, screen.Root, action.Mod,
			xproto.Keycode(action.Keycode),
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	}
	/*
		xproto.GrabKey(xconn, true, screen.Root, xproto.ModMaskControl, 36,
			xproto.GrabModeAsync, xproto.GrabModeAsync)
		xproto.GrabKey(xconn, true, screen.Root, xproto.ModMaskControl, 9,
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	*/
}

func main() {
	curTileMode = tiling.TileHori
	cfg = InitConfig()
	fmt.Println("############")
	fmt.Println(cfg)
	fmt.Println("############")
	var err error
	xconn, err = xgb.NewConnDisplay(":0")
	if err != nil {
		fmt.Println("Could not connect to X server")
		return
	}

	setup = xproto.Setup(xconn)
	screen = setup.DefaultScreen(xconn)
	curWs.Bounds = tiling.Area{
		X:      0,
		Y:      0,
		Width:  uint32(screen.WidthInPixels),
		Height: uint32(screen.HeightInPixels)}
	curWs.Root = &tiling.Tile{curTileMode, curWs.Bounds, 0, nil, nil} //give 0 or screen.Root?
	fmt.Printf("setup info vendor %s \n", setup.Vendor)

	fmt.Printf("screen height Width %d %d", screen.HeightInPixels,
		screen.WidthInPixels)
	getWMatoms()
	err = xproto.ChangeWindowAttributesChecked(
		xconn,
		screen.Root,
		xproto.CwEventMask,
		[]uint32{
			xproto.EventMaskKeyPress |
				xproto.EventMaskKeyPress |
				xproto.EventMaskKeyRelease |
				xproto.EventMaskButtonPress |
				xproto.EventMaskButtonRelease |
				xproto.EventMaskStructureNotify |
				xproto.EventMaskSubstructureRedirect |
				xproto.EventMaskEnterWindow |
				xproto.EventMaskLeaveWindow,
		}).Check()
	if err != nil {
		if _, ok := err.(xproto.AccessError); ok {
			fmt.Println("Could not become the WM. Is another WM already running?")
			panic(err)
		}
	}

	grabKeyEvents()
	//testing launching app
	go startApp("termite", "")
	// get existing windows and place them into our window structure
	queryWindows()
	//tile_windows()
	curWs.Config(xconn, 10, 10)
	for {
		// WaitForEvent either returns an event or an error and never both.
		// If both are nil, then something went wrong and the loop should be
		// halted.
		//
		// An error can only be seen here as a response to an unchecked
		// request.
		ev, xerr := xconn.WaitForEvent()
		if ev == nil && xerr == nil {
			fmt.Println("Both event and error are nil. Exiting...")
			return
		}

		if ev != nil {
			fmt.Printf("Event: %s\n", ev)
		}
		if xerr != nil {
			fmt.Printf("Error: %s\n", xerr)
		}
		switch e := ev.(type) {
		case xproto.KeyPressEvent:
			handleKeyPress(e)
		case xproto.ButtonPressEvent:
			handleButtonPress(e)
		case xproto.MapRequestEvent:
			fmt.Println("MapRequestEvent")
			if winattrib, err := xproto.GetWindowAttributes(xconn, e.Window).Reply(); err != nil || !winattrib.OverrideRedirect {
				xproto.MapWindowChecked(xconn, e.Window)
				mapWindow(e.Window)
				//tile_windows()
				curWs.Config(xconn, 10, 10)
			}
		case xproto.ConfigureRequestEvent:
			fmt.Println("ConfigureRequestEvent")
			ev := xproto.ConfigureNotifyEvent{
				Event:            e.Window,
				Window:           e.Window,
				AboveSibling:     0,
				X:                e.X,
				Y:                e.Y,
				Width:            e.Width,
				Height:           e.Height,
				BorderWidth:      0,
				OverrideRedirect: false,
			}
			xproto.SendEventChecked(xconn, false,
				e.Window, xproto.EventMaskStructureNotify, string(ev.Bytes()))
		case xproto.UnmapNotifyEvent:
			fmt.Println("UnmapNotifyEvent")
			//unmap_window(e.Window)
			//query_windows()
			//tile_windows()
		case xproto.DestroyNotifyEvent:
			unmapWindow(e.Window)
			//query_windows()
			//tile_windows()
			curWs.Config(xconn, 10, 10)
		case xproto.EnterNotifyEvent:
			if e.Event != screen.Root {
				currFocus = e.Event
				giveFocus(e.Event)
			}

		default:
		}

	}
}
