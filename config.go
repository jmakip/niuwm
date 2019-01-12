package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

//Hard coded config paths
const keyconfig_path string = "/.config/niuwm/keybinds.json"

type cmd_keybind struct {
	Cmd       string
	Cmdparams string
	Mod       uint16
	Keycode   byte
}
type logout_keybind struct {
	Mod     uint16
	Keycode byte
}
type action_keybind struct {
	Action  string
	Mod     uint16
	Keycode byte
}

type cmd_keys struct {
	Actions []action_keybind
	Cmd     []cmd_keybind
}

type niu_cfg struct {
	Keybinds cmd_keys
	//mouse bindings
	//workspaces etc...
}

func init_config() (cfg niu_cfg) {
	var err error
	cfg.Keybinds, err = load_keybindings()
	if err != nil {
		//trying to generate some defaults
		cfg.Keybinds = gen_keybinds()
	}
	return cfg
}

func load_keybindings() (ret cmd_keys, err error) {
	key_file, err := os.Open(os.Getenv("HOME") + keyconfig_path)
	if err != nil {
		fmt.Printf("Could not open config file: %s \n", keyconfig_path)
		return
	}
	defer key_file.Close()
	ascii, err := ioutil.ReadAll(key_file)
	if err != nil {
		fmt.Printf("Cant readl config file: %s \n", keyconfig_path)
	}
	err = json.Unmarshal(ascii, &ret)
	if err != nil {
		//file exists but cant translate it perhaps version mismatch
		fmt.Printf("Cant Unmarshal config file: %s \n", keyconfig_path)
	}
	return ret, err
}

//inconfiniance function for generating json
func gen_keybinds() (keys cmd_keys) {
	keys = cmd_keys{
		Actions: []action_keybind{
			action_keybind{
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
		Cmd: []cmd_keybind{
			cmd_keybind{Cmd: "termite", Cmdparams: "", Mod: 4, Keycode: 36},
			cmd_keybind{Cmd: "rofi", Cmdparams: "-show drun", Mod: 4, Keycode: 40},
		},
	}

	return keys
}
