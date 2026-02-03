package tcp_api

import (
	"fmt"
	"strings"

	"screenshot_server/Global"
	"screenshot_server/image_export"
	"screenshot_server/utils"
)

const imgExportDefaultDir = "./img_dump"

func Execute_img(safe_conn utils.Safe_connection, recv string) {
	recvList := strings.Fields(recv)
	if len(recvList) < 2 {
		writeImgResponse(safe_conn, "invalid img command")
		return
	}
	switch recvList[1] {
	case "count":
		executeImgCount(safe_conn, recvList[2:])
		return
	case "copy":
		executeImgCopy(safe_conn, recvList[2:])
		return
	default:
		writeImgResponse(safe_conn, "invalid img command")
		return
	}
}

func executeImgCount(safe_conn utils.Safe_connection, args []string) {
	if len(args) != 1 {
		writeImgResponse(safe_conn, "invalid img count command")
		return
	}
	tr, err := image_export.ParseRange(args[0])
	if err != nil {
		writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}
	count, err := image_export.CountImages(Global.Global_database_net, tr)
	if err != nil {
		writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}
	writeImgResponse(safe_conn, fmt.Sprintf("img count: %d", count))
}

func executeImgCopy(safe_conn utils.Safe_connection, args []string) {
	if len(args) < 1 {
		writeImgResponse(safe_conn, "invalid img copy command")
		return
	}
	tr, err := image_export.ParseRange(args[0])
	if err != nil {
		writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}

	dest := ""
	if len(args) > 1 {
		dest = strings.Join(args[1:], " ")
	}
	imgPath := Global.Global_constant_config.Img_path
	result, err := image_export.CopyImages(Global.Global_database_net, imgPath, dest, tr)
	if err != nil {
		writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}

	destOut := strings.TrimSpace(dest)
	if destOut == "" {
		destOut = imgExportDefaultDir
	}
	writeImgResponse(safe_conn, "img copy: "+result.Summary()+" dest="+destOut)
}

func writeImgResponse(safe_conn utils.Safe_connection, msg string) {
	safe_conn.Lock.Lock()
	safe_conn.Conn.Write([]byte(msg))
	safe_conn.Lock.Unlock()
}
