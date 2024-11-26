package main

import (
	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func startGUI() fyne.Window {
	// 创建 Fyne 应用
	myApp := app.New()
	myWindow := myApp.NewWindow("GUI with Buttons")

	// 创建显示当前 ss 值的标签
	label := widget.NewLabel("Shared Data: Waiting for input...")

	// 按钮 1：设置 ss 为 "1"
	button1 := widget.NewButton("Cancel Recording", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 0
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set cancel") // 更新标签
	})

	// 按钮 2：设置 ss 为 "2"
	button2 := widget.NewButton("Start Recording", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 1
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set start") // 更新标签
	})

	// 按钮 3：设置 ss 为 "3"
	button3 := widget.NewButton("Pause Recreding", func() {
		Global_sig_ss_Mutex.Lock()
		Globalsig_ss = 2
		Global_sig_ss_Mutex.Unlock()
		label.SetText("Set pause")
	})

	// 布局：将标签和按钮放入垂直容器
	content := container.NewVBox(
		label,
		button1,
		button2,
		button3,
	)

	// 设置窗口内容并显示
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(400, 200)) // 设置窗口大小
	return myWindow
}
