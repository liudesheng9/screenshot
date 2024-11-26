package main

import (
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func startGUI() fyne.Window {
	
	myApp := app.New()
	myWindow := myApp.NewWindow("GUI with Buttons")

	label := widget.NewLabel("Shared Data: Waiting for input...")

	button1 := widget.NewButton("Cancel Recording", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 0
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set cancel") // 更新标签
	})

	button2 := widget.NewButton("Start Recording", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 1
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set start") // 更新标签
	})

	button3 := widget.NewButton("Pause Recreding", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 2
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set pause")
	})

	content := container.NewVBox(
		label,
		button1,
		button2,
		button3,
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(400, 200)) // 设置窗口大小
	return myWindow
}
