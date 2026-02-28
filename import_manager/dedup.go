package import_manager

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

type dedupAction string

const (
	dedupInsert dedupAction = "insert"
	dedupUpdate dedupAction = "update"
	dedupSkip   dedupAction = "skip"
)

type sqlExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func hashStringSHA256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

func checkExists(db *sql.DB, fileName string) (exists bool, existingID string, err error) {
	if db == nil {
		return false, "", fmt.Errorf("database is nil")
	}

	fileID := hashStringSHA256(fileName)
	err = db.QueryRow(`SELECT id FROM screenshots WHERE id = ? OR file_name = ? LIMIT 1`, fileID, fileName).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, existingID, nil
}

func shouldInsertOrUpdate(fileID string, idExists bool, existingIDByFileName string) dedupAction {
	if idExists {
		return dedupSkip
	}
	if existingIDByFileName != "" && existingIDByFileName != fileID {
		return dedupUpdate
	}
	return dedupInsert
}

func lookupDedupState(exec sqlExecutor, fileID, fileName string) (bool, string, error) {
	var idExists bool
	err := exec.QueryRow(`SELECT EXISTS(SELECT 1 FROM screenshots WHERE id = ?)`, fileID).Scan(&idExists)
	if err != nil {
		return false, "", err
	}

	var existingID sql.NullString
	err = exec.QueryRow(`SELECT id FROM screenshots WHERE file_name = ? LIMIT 1`, fileName).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return idExists, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if !existingID.Valid {
		return idExists, "", nil
	}
	return idExists, existingID.String, nil
}

func metadataSQLValues(meta ImageMeta) (interface{}, interface{}) {
	if meta.HashKind == "" {
		return nil, nil
	}
	return strconv.FormatUint(meta.Hash, 10), meta.HashKind
}

func insertRecord(exec sqlExecutor, record importRecord) error {
	hashValue, hashKindValue := metadataSQLValues(record.Meta)
	_, err := exec.Exec(
		`INSERT INTO screenshots (id, hash, hash_kind, year, month, day, hour, minute, second, display_num, file_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.FileID,
		hashValue,
		hashKindValue,
		record.Meta.Year,
		record.Meta.Month,
		record.Meta.Day,
		record.Meta.Hour,
		record.Meta.Minute,
		record.Meta.Second,
		record.Meta.DisplayNum,
		record.FileName,
	)
	return err
}

func updateRecordByFileName(exec sqlExecutor, record importRecord) error {
	hashValue, hashKindValue := metadataSQLValues(record.Meta)
	_, err := exec.Exec(
		`UPDATE screenshots SET id = ?, hash = ?, hash_kind = ?, year = ?, month = ?, day = ?, hour = ?, minute = ?, second = ?, display_num = ?, file_name = ? WHERE file_name = ?`,
		record.FileID,
		hashValue,
		hashKindValue,
		record.Meta.Year,
		record.Meta.Month,
		record.Meta.Day,
		record.Meta.Hour,
		record.Meta.Minute,
		record.Meta.Second,
		record.Meta.DisplayNum,
		record.FileName,
		record.FileName,
	)
	return err
}

func applyRecord(exec sqlExecutor, record importRecord) (dedupAction, error) {
	idExists, existingIDByFileName, err := lookupDedupState(exec, record.FileID, record.FileName)
	if err != nil {
		return "", err
	}

	action := shouldInsertOrUpdate(record.FileID, idExists, existingIDByFileName)
	switch action {
	case dedupSkip:
		return dedupSkip, nil
	case dedupUpdate:
		if err := updateRecordByFileName(exec, record); err != nil {
			return "", err
		}
		return dedupUpdate, nil
	default:
		if err := insertRecord(exec, record); err != nil {
			return "", err
		}
		return dedupInsert, nil
	}
}
