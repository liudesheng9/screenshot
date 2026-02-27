package tcp_api

import (
	"fmt"
	"net"
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

type WorkerStatus struct {
	WorkerID    int
	WorkerLabel string
	Count       int
	Filename    string
	Stage       string
	Elapsed     string
}

type ProgressUpdateV2 struct {
	Total          int
	Target         int
	WorkerStatuses []WorkerStatus
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
	enableNoDelay(safe_conn.Conn)

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
			if err := writeImgLine(safe_conn, formatProgressLineV2(toProgressUpdateV2(update))); err != nil {
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

func toProgressUpdateV2(update image_export.ProgressUpdate) ProgressUpdateV2 {
	workerIDs := make([]int, 0, len(update.WorkerCounts)+len(update.WorkerTasks)+len(update.WorkerRefs))
	seenWorkerID := make(map[int]struct{}, len(update.WorkerCounts)+len(update.WorkerTasks)+len(update.WorkerRefs))
	appendWorkerID := func(workerID int) {
		if _, seen := seenWorkerID[workerID]; seen {
			return
		}
		seenWorkerID[workerID] = struct{}{}
		workerIDs = append(workerIDs, workerID)
	}
	for workerID := range update.WorkerCounts {
		appendWorkerID(workerID)
	}
	for workerID := range update.WorkerTasks {
		appendWorkerID(workerID)
	}
	for workerID := range update.WorkerRefs {
		appendWorkerID(workerID)
	}

	sort.Slice(workerIDs, func(i, j int) bool {
		leftRef := resolveWorkerRef(update, workerIDs[i])
		rightRef := resolveWorkerRef(update, workerIDs[j])
		leftRank := workerTypeRank(leftRef.WorkerType)
		rightRank := workerTypeRank(rightRef.WorkerType)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if leftRef.WorkerID != rightRef.WorkerID {
			return leftRef.WorkerID < rightRef.WorkerID
		}
		return workerIDs[i] < workerIDs[j]
	})

	statuses := make([]WorkerStatus, 0, len(workerIDs))
	for _, internalWorkerID := range workerIDs {
		workerRef := resolveWorkerRef(update, internalWorkerID)
		status := WorkerStatus{
			WorkerID:    workerRef.WorkerID,
			WorkerLabel: workerRef.Label(),
			Count:       update.WorkerCounts[internalWorkerID],
			Filename:    "-",
			Stage:       "idle",
			Elapsed:     "-",
		}

		if workerTask, ok := update.WorkerTasks[internalWorkerID]; ok && workerTask != nil {
			if workerTask.Filename != "" {
				status.Filename = workerTask.Filename
			}
			if workerTask.Stage != "" {
				status.Stage = string(workerTask.Stage)
			}
			status.Elapsed = formatElapsedSeconds(update.Timestamp, workerTask.StartTime)
		}

		statuses = append(statuses, status)
	}

	return ProgressUpdateV2{
		Total:          update.Total,
		Target:         update.Target,
		WorkerStatuses: statuses,
	}
}

func resolveWorkerRef(update image_export.ProgressUpdate, workerID int) image_export.WorkerRef {
	if ref, ok := update.WorkerRefs[workerID]; ok {
		return ref
	}
	if task, ok := update.WorkerTasks[workerID]; ok && task != nil {
		return image_export.WorkerRef{WorkerType: task.WorkerType, WorkerID: workerID}
	}
	return image_export.InferWorkerRef(workerID)
}

func workerTypeRank(workerType image_export.WorkerType) int {
	switch workerType {
	case image_export.WorkerTypeIO:
		return 0
	case image_export.WorkerTypePROC:
		return 1
	default:
		return 2
	}
}

func formatProgressLineV2(update ProgressUpdateV2) string {
	parts := make([]string, 0, len(update.WorkerStatuses)+2)
	parts = append(parts, "PROGRESS_V2")
	parts = append(parts, fmt.Sprintf("T:%d/%d", update.Total, update.Target))
	for _, status := range update.WorkerStatuses {
		workerLabel := strings.TrimSpace(status.WorkerLabel)
		if workerLabel == "" {
			workerLabel = fmt.Sprintf("W%d", status.WorkerID)
		}
		parts = append(parts, fmt.Sprintf(
			"%s:%d:%s:%s:%s",
			normalizeProgressField(workerLabel, fmt.Sprintf("W%d", status.WorkerID)),
			status.Count,
			normalizeProgressField(status.Filename, "-"),
			normalizeProgressField(status.Stage, "idle"),
			normalizeProgressField(status.Elapsed, "-"),
		))
	}
	return strings.Join(parts, " ")
}

func normalizeProgressField(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	replacer := strings.NewReplacer(" ", "_", ":", "_")
	return replacer.Replace(trimmed)
}

func formatElapsedSeconds(now, start time.Time) string {
	if start.IsZero() {
		return "-"
	}
	elapsed := now.Sub(start)
	if elapsed < 0 {
		elapsed = 0
	}
	seconds := elapsed.Seconds()
	if seconds < 10 {
		return fmt.Sprintf("%.1f", seconds)
	}
	return fmt.Sprintf("%.0f", seconds)
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

func enableNoDelay(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
}
