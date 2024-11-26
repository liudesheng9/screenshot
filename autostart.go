package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func pause() {
	fmt.Println("--------------------------------------")
	fmt.Printf("Press any key to continue...")
	b := make([]byte, 1)
	os.Stdin.Read(b)
}

func setAutoStart(appName, appPath string) error {
	fmt.Printf("%s %s", appName, appPath)
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.SetStringValue(appName, appPath)
	if err != nil {
		fmt.Println("Unable to set autostart")
		pause()
		panic(err)
	}
	fmt.Println("Autostart set successfully")
	return nil
	/*
		_, _, err = key.GetStringValue(appName)
		if errors.Is(err, registry.ErrNotExist) {
			// key does not exist
			err = key.SetStringValue(appName, appPath)
			if err != nil {
				pause()
				panic(err)
			}
			fmt.Println("Key set successfully")
			return nil
		} else if err != nil {
			fmt.Println("Unable to get")
			pause()
			panic(err)
		} else {
			// key exists
			fmt.Println("Key exists")
			pause()
			return nil
		}
	*/
}

func getAppPath() (string, string) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	// App name get
	appName := filepath.Base(ex)
	return ex, appName
}

func autostartInit() {
	var appName, appPath string
	appPath, appName = getAppPath()

	err := setAutoStart(appName, appPath)
	if err != nil {
		log.Println(err)
		panic(err)
	}
}
