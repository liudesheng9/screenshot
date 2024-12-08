package init_config

import (
	"fmt"
	"os"
	"screenshot_server/utils"

	"github.com/BurntSushi/toml"
)

func Init_ss_constant_config_from_toml() utils.Ss_constant_config {
	var c utils.Ss_constant_config
	fp, err := os.Open("./config.toml")
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
