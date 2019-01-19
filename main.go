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
var event_timestamp xproto.Timestamp
var windows []xproto.Window
var wm_atoms map[string]xproto.Atom
var cfg niu_cfg

var curr_mode uint16
var curr_ws workspace

type area struct {
	x      uint32
	y      uint32
	size_x uint32
	size_y uint32
}

type workspace struct {
	name    string
	bounds  area
	storage *Container
}

type Container struct {
	mode     uint16
	bounds   area
	windows  []xproto.Window
	sub_area *Container
}

func TileContainers(cont *Container) {
	if cont == nil {
		return
	}
	if len(cont.windows) != 0 {
		TileContainer(cont)
	}
	if cont.sub_area != nil {
		TileContainers(cont.sub_area)
	}
}

func TileContainer(cont *Container) {
	var pad uint32 = 10
	var screen_x uint32 = cont.bounds.size_x
	var screen_y uint32 = cont.bounds.size_y
	var nro_windows uint32 = uint32(len(cont.windows))
	var tile_size_x uint32
	var tile_size_y uint32
	var x_off uint32
	var y_off uint32
	if nro_windows == 0 {
		return
	}
	if cont.mode == 1 {
		tile_size_y = screen_y - pad*2
		tile_size_x = (screen_x / (nro_windows)) - pad*2
		x_off = pad + tile_size_x + pad
		y_off = 0
	}
	if cont.mode == 0 {
		tile_size_y = (screen_y / (nro_windows)) - pad*2
		tile_size_x = screen_x - pad*2
		x_off = 0
		y_off = pad + tile_size_y + pad
	}
	for index, wind := range cont.windows {
		var idx uint32 = uint32(index)

		x := pad + (x_off)*uint32(idx) + cont.bounds.x
		y := pad + (y_off)*uint32(idx) + cont.bounds.y
		err := xproto.ConfigureWindowChecked(xconn, wind, xproto.ConfigWindowX|
			xproto.ConfigWindowY|
			xproto.ConfigWindowWidth|
			xproto.ConfigWindowHeight,
			[]uint32{x, y, tile_size_x, tile_size_y}).Check()
		if err != nil {
			fmt.Println(err)
		}
	}
}
func RemoveWindow(wind xproto.Window) {
	var pcont *Container
	var ccont *Container
	var nw []xproto.Window
	//find container
	for cont := curr_ws.storage; cont != nil; cont = cont.sub_area {
		for _, w := range cont.windows {
			if w == wind {
				ccont = cont
				goto end_loop
			}
		}
		pcont = cont
	}
end_loop:
	if len(ccont.windows) > 1 {
		for _, w := range ccont.windows {
			if w != wind {
				nw = append(nw, w)
			}
		}
		ccont.windows = nw
		return
	}
	//remove container
	if pcont == nil {
		curr_ws.storage = ccont.sub_area
		if ccont.sub_area != nil {
			ccont.sub_area.bounds.size_x += ccont.bounds.size_x
			ccont.sub_area.bounds.size_y += ccont.bounds.size_y
			ccont.sub_area.bounds.x = ccont.bounds.x
			ccont.sub_area.bounds.y = ccont.bounds.y
		}
		return
	}
	if ccont.sub_area == nil {
		pcont.sub_area = nil
		pcont.bounds.size_x += ccont.bounds.size_x
		pcont.bounds.size_y += ccont.bounds.size_y
		return
	}
	//hardest case
	//if parent has decreased size its best to realloc bounds size
	//TODO i just now give it to parent but once i have splits on mid of list
	//need to implement checks
	pcont.sub_area = ccont.sub_area
	pcont.bounds.size_x += ccont.bounds.size_x
	pcont.bounds.size_y += ccont.bounds.size_y

}
func AddWindow(window xproto.Window) {
	curr_focus := focused_window()
	if curr_ws.storage == nil {
		cont := Container{
			mode:     curr_mode,
			bounds:   curr_ws.bounds,
			windows:  []xproto.Window{window},
			sub_area: nil}

		curr_ws.storage = &cont //Container{cont}
		return
	}

	if curr_focus == 0 {
		if curr_ws.storage != nil && len(curr_ws.storage.windows) != 0 {
			curr_focus = curr_ws.storage.windows[0]
		}
	}
	var ccont *Container
	//find Container
	/*
			for cont := curr_ws.storage; cont != nil; cont = cont.sub_area {
				for _, wind := range cont.windows {
					if wind == curr_focus {
						ccont = cont
						goto end_loop
					}
				}
			}
		end_loop:
	*/
	ccont = curr_ws.storage
	for ccont.sub_area != nil {
		ccont = ccont.sub_area
	}
	if ccont == nil {
		ccont = curr_ws.storage
	}
	if curr_mode == ccont.mode {
		ccont.windows = append(ccont.windows, window)
		return
	}
	var c_bounds area
	var n_bounds area
	if curr_mode == 0 {
		c_bounds = area{x: ccont.bounds.x,
			y:      ccont.bounds.y,
			size_x: ccont.bounds.size_x,
			size_y: ccont.bounds.size_y / 2}
		n_bounds = area{
			x:      c_bounds.x,
			y:      c_bounds.y + c_bounds.size_y,
			size_x: c_bounds.size_x,
			size_y: c_bounds.size_y}
	}
	if curr_mode == 1 {
		c_bounds = area{x: ccont.bounds.x,
			y:      ccont.bounds.y,
			size_x: ccont.bounds.size_x / 2,
			size_y: ccont.bounds.size_y}
		n_bounds = area{
			x:      c_bounds.x + c_bounds.size_x,
			y:      c_bounds.y,
			size_x: c_bounds.size_x,
			size_y: c_bounds.size_y}
	}
	ccont.bounds = c_bounds
	box := Container{mode: curr_mode,
		bounds:   n_bounds,
		windows:  []xproto.Window{window},
		sub_area: nil}
	//curr_ws.storage = append(curr_ws.storage, box)
	ccont.sub_area = &box

}

//try to find window that is active
//TODO there must be some better way to do this
func focused_window() xproto.Window {
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
func give_focus(w xproto.Window) {
	err := xproto.SetInputFocus(xconn, xproto.InputFocusPointerRoot,
		w, event_timestamp).Check()
	if err != nil {
		fmt.Printf("could not get active window id\n")
	}

}

//now just move inside windows slice,
func move_focus_left() {
	if len(windows) < 1 {
		return
	}

	active := focused_window()
	if active == 0 {
		//no focused_window just give it to first
		if len(windows) > 0 {
			give_focus(windows[0])
		}
		return
	}
	for i, window := range windows {
		if active == window {
			if i > 0 {
				give_focus(windows[i-1])
				return
			}
			//rotate to last if we are at start
			give_focus(windows[len(windows)-1])
		}
	}
	return

}

//now just move inside windows slice, later add Containers
func move_focus_right() {
	if len(windows) < 1 {
		return
	}
	active := focused_window()
	if active == 0 {
		give_focus(windows[len(windows)-1])
		return
	}
	for i, window := range windows {
		if active == window {
			if i < len(windows)-1 {
				give_focus(windows[i+1])
				return
			}
			//rotate to last if we are at start
			give_focus(windows[0])
		}
	}
	return
}

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

//Start application with parameters wait for it to finish
func start_app(name string, params string) {
	var full string = name + " " + params
	//TODO read shell env from config
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
			if actions.Action == "focus_left" {
				move_focus_left()
			}
			if actions.Action == "focus_right" {
				move_focus_right()
			}
			if actions.Action == "tiling_mode_toggle" {
				curr_mode = curr_mode ^ 1
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
				xproto.EventMaskEnterWindow}).Check(); err != nil {

		return
	}
	if window != screen.Root {
		//windows = append(windows, window)
		AddWindow(window)
	}
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
	RemoveWindow(window)
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
		err := xproto.ConfigureWindowChecked(xconn, wind,
			xproto.ConfigWindowX|
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
		xproto.GrabKey(xconn, true, screen.Root, cmd_.Mod,
			xproto.Keycode(cmd_.Keycode),
			xproto.GrabModeAsync, xproto.GrabModeAsync)
	}
	for _, action_ := range cfg.Keybinds.Actions {
		xproto.GrabKey(xconn, true, screen.Root, action_.Mod,
			xproto.Keycode(action_.Keycode),
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
	curr_mode = 1
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
	curr_ws.bounds = area{
		x:      0,
		y:      0,
		size_x: uint32(screen.WidthInPixels),
		size_y: uint32(screen.HeightInPixels)}
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
	//tile_windows()
	TileContainers(curr_ws.storage)
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
				//tile_windows()
				TileContainers(curr_ws.storage)
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
			//tile_windows()
			TileContainers(curr_ws.storage)
		default:
		}

	}
	fmt.Println("everything ok exiting")
}
