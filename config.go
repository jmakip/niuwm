package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

//Hard coded config paths
const keyConfigPath string = "/.config/niuwm/keybinds.json"

//CmdKeybind keybinds for executables
type CmdKeybind struct {
	Cmd       string
	Cmdparams string
	Mod       uint16
	Keycode   byte
}

//LogoutKeybind keycode for exiting WM
type LogoutKeybind struct {
	Mod     uint16
	Keycode byte
}

//ActionKeybind WM actions keybinds
type ActionKeybind struct {
	Action  string
	Mod     uint16
	Keycode byte
}

//CmdKeys all keybinds
type CmdKeys struct {
	Actions []ActionKeybind
	Cmd     []CmdKeybind
}

//NiuCfg settings load from config files
type NiuCfg struct {
	Keybinds CmdKeys
	//mouse bindings
	//workspaces etc...
}

//InitConfig initialize settings by loading from file
func InitConfig() (cfg NiuCfg) {
	var err error
	cfg.Keybinds, err = LoadKeyBindings()
	if err != nil {
		//trying to generate some defaults
		cfg.Keybinds = GenKeyBinds()
	}
	return cfg
}

//LoadKeyBindings load keybindings from JSON config files
func LoadKeyBindings() (ret CmdKeys, err error) {
	keyFile, err := os.Open(os.Getenv("HOME") + keyConfigPath)
	if err != nil {
		fmt.Printf("Could not open config file: %s \n", keyConfigPath)
		return
	}
	defer keyFile.Close()
	ascii, err := ioutil.ReadAll(keyFile)
	if err != nil {
		fmt.Printf("Cant readl config file: %s \n", keyConfigPath)
	}
	err = json.Unmarshal(ascii, &ret)
	if err != nil {
		//file exists but cant translate it perhaps version mismatch
		fmt.Printf("Cant Unmarshal config file: %s \n", keyConfigPath)
	}
	return ret, err
}

//GenKeyBinds generate default config for keybinds, use when file does not exists.
func GenKeyBinds() (keys CmdKeys) {
	keys = CmdKeys{
		Actions: []ActionKeybind{
			ActionKeybind{
				Action:  "logout",
				Mod:     4,
				Keycode: 9,
			},
			{
				Action:  "unknown",
				Mod:     0xff,
				Keycode: 69,
			},
		},
		Cmd: []CmdKeybind{
			CmdKeybind{Cmd: "termite", Cmdparams: "", Mod: 4, Keycode: 36},
			CmdKeybind{Cmd: "rofi", Cmdparams: "-show drun", Mod: 4, Keycode: 40},
		},
	}

	return keys
}
