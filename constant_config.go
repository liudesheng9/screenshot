package main

type ss_constant_config struct {
	cache_path        string
	img_path          string
	database_path     string
	screenshot_second int
}

func (c *ss_constant_config) init_ss_constant_config() {
	c.cache_path = "./cache"
	c.img_path = "./img"
	c.database_path = "./example.db"
	c.screenshot_second = 2
}

/*
func decode_toml_file(path string, v any) (toml.MetaData, error) {
	fp, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return toml.MetaData{}, err
	}
	defer fp.Close()
	return toml.NewDecoder(fp).Decode(v)
}

func init_ss_constant_config_from_toml() ss_constant_config {
	var c ss_constant_config
	fmt.Println(c)
	fp, err := os.Open("./config.toml")
	if err != nil {
		// c.init_ss_constant_config()
		fmt.Println(err)
		return c
	}
	defer fp.Close()
	var meta toml.MetaData
	meta, err = toml.NewDecoder(fp).Decode(&c)
	fmt.Println(meta)
	fmt.Println(err)
	fmt.Println(c)

	if err != nil {
		c.init_ss_constant_config()
	}
	if c == (ss_constant_config{}) {
		fmt.Println(os.Stat("./config.toml"))
		c.init_ss_constant_config()
	}
	return c
}
*/
