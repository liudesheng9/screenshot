package library_manager

import (
	"screenshot_server/Global"
)

func Check_if_locked(filename_list []string) bool {
	map_lock := make(map[string]bool)
	Global.Global_safe_file_lock.Lock.Lock()
	for _, item := range Global.Global_safe_file_lock.File_lock {
		map_lock[item] = true
	}
	Global.Global_safe_file_lock.Lock.Unlock()
	for _, filename := range filename_list {
		if !map_lock[filename] {
			return false
		}
	}
	return true
}

func Remove_lock(filename_list []string) {
	Global.Global_safe_file_lock.Lock.Lock()
	for _, filename := range filename_list {
		for i, item := range Global.Global_safe_file_lock.File_lock {
			if item == filename {
				Global.Global_safe_file_lock.File_lock = append(Global.Global_safe_file_lock.File_lock[:i], Global.Global_safe_file_lock.File_lock[i+1:]...)
				break
			}
		}
	}
	Global.Global_safe_file_lock.Lock.Unlock()
}
