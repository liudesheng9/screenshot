package Global

import (
	"database/sql"
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
