package tcp_api

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"screenshot_server/Global"
	"screenshot_server/image_export"
	"screenshot_server/utils"
)

const (
	imgExportDefaultDir = "./img_dump"
	imgCopyStreamFlag   = "--stream"
)

type ProgressUpdate struct {
	WorkerCounts map[int]int
	Timestamp    time.Time
}

type copyOutcome struct {
	result image_export.CopyResult
	err    error
}

func Execute_img(safe_conn utils.Safe_connection, recv string) {
	recvList := strings.Fields(recv)
	if len(recvList) < 2 {
		_ = writeImgResponse(safe_conn, "invalid img command")
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
		_ = writeImgResponse(safe_conn, "invalid img command")
		return
	}
}

func executeImgCount(safe_conn utils.Safe_connection, args []string) {
	if len(args) != 1 {
		_ = writeImgResponse(safe_conn, "invalid img count command")
		return
	}
	tr, err := image_export.ParseRange(args[0])
	if err != nil {
		_ = writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}
	imgPath := Global.Global_constant_config.Img_path
	count, err := image_export.CountImages(Global.Global_database_net, imgPath, tr)
	if err != nil {
		_ = writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}
	_ = writeImgResponse(safe_conn, fmt.Sprintf("img count: %s", count.Summary()))
}

func executeImgCopy(safe_conn utils.Safe_connection, args []string) {
	if len(args) < 1 {
		_ = writeImgResponse(safe_conn, "invalid img copy command")
		return
	}
	tr, err := image_export.ParseRange(args[0])
	if err != nil {
		_ = writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}

	dest, streamProgress := parseCopyArgs(args[1:])
	imgPath := Global.Global_constant_config.Img_path
	destOut := resolveDestOutput(dest)

	if streamProgress {
		streamImgCopy(safe_conn, tr, imgPath, dest, destOut)
		return
	}

	result, err := image_export.CopyImages(Global.Global_database_net, imgPath, dest, tr)
	if err != nil {
		_ = writeImgResponse(safe_conn, "img error: "+err.Error())
		return
	}
	_ = writeImgResponse(safe_conn, formatDoneLine(result, destOut))
}

func parseCopyArgs(args []string) (string, bool) {
	streamProgress := false
	destParts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == imgCopyStreamFlag {
			streamProgress = true
			continue
		}
		destParts = append(destParts, arg)
	}
	return strings.Join(destParts, " "), streamProgress
}

func streamImgCopy(
	safe_conn utils.Safe_connection,
	tr image_export.TimeRange,
	imgPath string,
	dest string,
	destOut string,
) {
	progressChan := make(chan image_export.ProgressUpdate, 64)
	resultChan := make(chan copyOutcome, 1)

	go func() {
		result, err := image_export.CopyImagesWithProgress(
			Global.Global_database_net,
			imgPath,
			dest,
			tr,
			progressChan,
		)
		resultChan <- copyOutcome{result: result, err: err}
	}()

	progressOpen := true
	var finalOutcome *copyOutcome
	for progressOpen || finalOutcome == nil {
		select {
		case update, ok := <-progressChan:
			if !ok {
				progressOpen = false
				continue
			}
			if err := writeImgLine(safe_conn, formatProgressLine(toProgressUpdate(update))); err != nil {
				return
			}
		case outcome := <-resultChan:
			finalOutcome = &outcome
		}
	}

	if finalOutcome.err != nil {
		_ = writeImgLine(safe_conn, "img error: "+finalOutcome.err.Error())
		return
	}
	_ = writeImgLine(safe_conn, formatDoneLine(finalOutcome.result, destOut))
}

func resolveDestOutput(dest string) string {
	destOut := strings.TrimSpace(dest)
	if destOut == "" {
		return imgExportDefaultDir
	}
	return destOut
}

func toProgressUpdate(update image_export.ProgressUpdate) ProgressUpdate {
	return ProgressUpdate{
		WorkerCounts: update.WorkerCounts,
		Timestamp:    update.Timestamp,
	}
}

func formatProgressLine(update ProgressUpdate) string {
	workerIDs := make([]int, 0, len(update.WorkerCounts))
	total := 0
	for workerID, count := range update.WorkerCounts {
		workerIDs = append(workerIDs, workerID)
		total += count
	}
	sort.Ints(workerIDs)

	parts := make([]string, 0, len(workerIDs)+2)
	parts = append(parts, "PROGRESS")
	for _, workerID := range workerIDs {
		parts = append(parts, fmt.Sprintf("W%d:%d", workerID, update.WorkerCounts[workerID]))
	}
	parts = append(parts, fmt.Sprintf("T:%d", total))
	return strings.Join(parts, " ")
}

func formatDoneLine(result image_export.CopyResult, destOut string) string {
	return fmt.Sprintf(
		"DONE copied=%d exist=%d failed=%d skipped=%d dest=%s",
		result.Copied,
		result.Existing,
		result.Failed,
		result.Skipped,
		destOut,
	)
}

func writeImgLine(safe_conn utils.Safe_connection, msg string) error {
	return writeImgResponse(safe_conn, msg+"\n")
}

func writeImgResponse(safe_conn utils.Safe_connection, msg string) error {
	safe_conn.Lock.Lock()
	defer safe_conn.Lock.Unlock()
	_, err := safe_conn.Conn.Write([]byte(msg))
	return err
}
