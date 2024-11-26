package main

type ss_constant_config struct {
	cache_path    string
	img_path      string
	database_path string
}

func (c *ss_constant_config) init_ss_constant_config() {
	c.cache_path = "./cache"
	c.img_path = "./img"
	c.database_path = "./example.db"
}
