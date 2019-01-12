package main

import (
	"fmt"
	"log"
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
var windows []xproto.Window
var wm_atoms map[string]xproto.Atom
var cfg niu_cfg

func find_window(w xproto.Window) (ret bool) {
	ret = false
	for _, window := range windows {
		if w == window {
			ret = true
			break
		}
	}
	return
}

//start application with parameters waitfor it to finish
func start_app(name string, params string) {
	var full string = name + " " + params
	cmd := exec.Command("/bin/bash", "-c", full)
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Started %s waiting to finish\n", name)
	err = cmd.Wait()
}

//pushes 3 atom requests to x server and then waits for them
func get_wm_atoms() {
	wm_atoms = make(map[string]xproto.Atom)
	atom_names := [3]string{"WM_PROTOCOLS", "WM_TAKE_FOCUS", "WM_DELETE_WINDOW"}
	var cookies [3]xproto.InternAtomCookie
	for i := 0; i < len(atom_names); i++ {
		cookies[i] = xproto.InternAtom(xconn, false, uint16(len(atom_names[i])),
			atom_names[i])
	}

	for i := 0; i < len(atom_names); i++ {
		reply, err := cookies[i].Reply()
		if err != nil {
			log.Fatal("err")
			panic(err)
		}
		if reply == nil {
			wm_atoms[atom_names[i]] = 0
			continue
		}
		wm_atoms[atom_names[i]] = reply.Atom
		fmt.Printf("success getting wm atoms")
	}

}

func handle_button_press(e xproto.ButtonPressEvent) {
	if e.State&xproto.ModMaskControl != 0 {
		go start_app("termite", "")
	}

	button := e.Detail
	if button == 9 {
		//close WM TODO proper now i just kill it
		panic("closing")
	}
	if button == 36 {
		go start_app("termite", "")
	}

}
func handle_key_press(e xproto.KeyPressEvent) {
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
	for _, cmd_ := range cfg.Keybinds.Cmd {
		if mod == cmd_.Mod && keycode == cmd_.Keycode {
			go start_app(cmd_.Cmd, cmd_.Cmdparams)
			return
		}
	}
	for _, actions := range cfg.Keybinds.Actions {
		if mod == actions.Mod && keycode == actions.Keycode {
			if actions.Action == "logout" {
				panic("closing")
			}
		}
	}

}

func map_window(window xproto.Window) {
	if find_window(window) != false {
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
				xproto.EventMaskEnterWindow,
		}).Check(); err != nil {
		return
	}
	if window != screen.Root {
		windows = append(windows, window)
	}
	fmt.Printf("Windows: ")
	fmt.Println(windows)
}
func unmap_window(window xproto.Window) {
	//todo remove window from slicea
	var nw []xproto.Window
	for _, w := range windows {
		if w != window {
			nw = append(nw, w)
		}
	}
	windows = nw
	fmt.Printf("Windows: ")
	fmt.Println(windows)
}

func query_windows() {
	//perhaps remove all from windows first
	windows = nil
	tree, err := xproto.QueryTree(xconn, screen.Root).Reply()
	if err != nil {
		panic(err)
	}
	if tree != nil {
		for i, wind := range tree.Children {
			if i > 0 {
				map_window(wind)
			}
		}
	}

}
func tile_windows() {
	var pad uint32 = 10
	var screen_x uint32 = uint32(screen.WidthInPixels)
	var screen_y uint32 = uint32(screen.HeightInPixels)
	var nro_windows uint32 = uint32(len(windows))
	var tile_size_x uint32
	if nro_windows == 0 {
		return
	}
	if nro_windows == 1 {
		tile_size_x = screen_x - pad
	} else {
		tile_size_x = (screen_x / (nro_windows)) - pad
	}
	for index, wind := range windows {
		var idx uint32 = uint32(index)

		x := pad + (pad+tile_size_x+pad)*uint32(idx)
		err := xproto.ConfigureWindowChecked(xconn, wind, xproto.ConfigWindowX|
			xproto.ConfigWindowY|
			xproto.ConfigWindowWidth|
			xproto.ConfigWindowHeight,
			[]uint32{x, 0, tile_size_x, screen_y}).Check()
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Printf("NRO WINDOWS %d \n", len(windows))
}

func grab_key_events() {
	for _, cmd_ := range cfg.Keybinds.Cmd {
		xproto.GrabKey(xconn, true, screen.Root, cmd_.Mod, xproto.Keycode(cmd_.Keycode),
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	}
	for _, action_ := range cfg.Keybinds.Actions {
		xproto.GrabKey(xconn, true, screen.Root, action_.Mod, xproto.Keycode(action_.Keycode),
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
	cfg = init_config()
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

	fmt.Printf("setup info vendor %s \n", setup.Vendor)

	fmt.Printf("screen height Width %d %d", screen.HeightInPixels,
		screen.WidthInPixels)
	get_wm_atoms()
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
				xproto.EventMaskSubstructureRedirect,
		}).Check()
	if err != nil {
		if _, ok := err.(xproto.AccessError); ok {
			fmt.Println("Could not become the WM. Is another WM already running?")
			panic(err)
		}
	}

	grab_key_events()
	//testing launching app
	go start_app("termite", "")
	// get existing windows and place them into our window structure
	query_windows()
	tile_windows()
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
			handle_key_press(e)
		case xproto.ButtonPressEvent:
			handle_button_press(e)
		case xproto.MapRequestEvent:
			fmt.Println("MapRequestEvent")
			if winattrib, err := xproto.GetWindowAttributes(xconn, e.Window).Reply(); err != nil || !winattrib.OverrideRedirect {
				xproto.MapWindowChecked(xconn, e.Window)
				map_window(e.Window)
				tile_windows()
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
			unmap_window(e.Window)
			//query_windows()
			tile_windows()
		default:
		}

	}
	fmt.Println("everything ok exiting")
}
