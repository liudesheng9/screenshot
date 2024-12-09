package init_config

import (
	"fmt"
	"os"
	"screenshot_server/utils"

	"github.com/BurntSushi/toml"
)

func Init_ss_constant_config_from_toml(toml_path string) utils.Ss_constant_config {
	var c utils.Ss_constant_config
	fp, err := os.Open(toml_path)
	if err != nil {
		// c.init_ss_constant_config()
		fmt.Println("Open toml failed: ", err)
		return c
	}
	defer fp.Close()
	_, err = toml.NewDecoder(fp).Decode(&c)

	if err != nil {
		fmt.Println("Init from toml failed: ", err)
		c.Init_ss_constant_config()
	}
	if c == (utils.Ss_constant_config{}) {
		c.Init_ss_constant_config()
	}
	return c
}

func Encode_ss_constant_config_to_toml(c utils.Ss_constant_config, toml_path string) {
	fp, err := os.OpenFile(toml_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Create toml failed: ", err)
		return
	}
	defer fp.Close()
	err = toml.NewEncoder(fp).Encode(c)
	if err != nil {
		fmt.Println("Encode toml failed: ", err)
	}
}
