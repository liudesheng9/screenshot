package Global

import (
	"database/sql"
	"image"
	"os"
	"screenshot_server/utils"
	"sync"
)

var Globalsig_ss *int
var Global_sig_ss_Mutex *sync.Mutex

var Global_constant_config *utils.Ss_constant_config
var Global_screenshot_gap_Mutex *sync.Mutex
var Global_cache_path_Mutex *sync.Mutex
var Global_cache_path_instant_Mutex *sync.Mutex

var Global_database *sql.DB
var Global_database_net *sql.DB

var Global_logFile *os.File

var Global_safe_file_lock *utils.Safe_file_lock

var Global_map_image map[int]map[int64]*image.RGBA
var Global_map_image_Mutex *sync.Mutex

// state identitier
var Global_screenshot_status int
var Global_screenshot_status_Mutex *sync.Mutex
