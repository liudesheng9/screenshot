package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	pis "github.com/dsoprea/go-png-image-structure/v2"
)

type ImageMacro struct {
	hash         uint64
	hashKind     string
	year         int
	month        int
	day          int
	hour         int
	minute       int
	second       int
	displayNum   int
	AlphaMessage string
}

func init_macro(fileName string, img *image.RGBA) ImageMacro {
	macro := ImageMacro{}
	hash, _ := AverageHash(img)
	imageHash := hash.hash
	imageHashKind := hash.kind
	dateStr := strings.Split(fileName, "_")[0]
	timeStr := strings.Split(fileName, "_")[1]
	displayNum, _ := strconv.Atoi(strings.Split(fileName, "_")[2])
	imageYear, _ := strconv.Atoi(dateStr[:4])
	imageMonth, _ := strconv.Atoi(dateStr[4:6])
	imageDay, _ := strconv.Atoi(dateStr[6:8])
	imageHour, _ := strconv.Atoi(timeStr[:2])
	imageMinute, _ := strconv.Atoi(timeStr[2:4])
	imageSecond, _ := strconv.Atoi(timeStr[4:6])
	AlphaMessage := "This screenshot owned by Chen, Xingtong"

	macro.hash = imageHash
	macro.hashKind = imageHashKind
	macro.year = imageYear
	macro.month = imageMonth
	macro.day = imageDay
	macro.hour = imageHour
	macro.minute = imageMinute
	macro.second = imageSecond
	macro.displayNum = displayNum
	macro.AlphaMessage = AlphaMessage

	return macro
}

func convert_macro_to_map(macro ImageMacro) map[string]string {
	macroMap := make(map[string]string)
	macroMap["hash"] = fmt.Sprintf("%d", macro.hash)
	macroMap["hashKind"] = macro.hashKind
	macroMap["year"] = fmt.Sprintf("%d", macro.year)
	macroMap["month"] = fmt.Sprintf("%d", macro.month)
	macroMap["day"] = fmt.Sprintf("%d", macro.day)
	macroMap["hour"] = fmt.Sprintf("%d", macro.hour)
	macroMap["minute"] = fmt.Sprintf("%d", macro.minute)
	macroMap["second"] = fmt.Sprintf("%d", macro.second)
	macroMap["displayNum"] = fmt.Sprintf("%d", macro.displayNum)
	macroMap["AlphaMessage"] = macro.AlphaMessage

	return macroMap
}

func convert_macro_to_interface_map(macro ImageMacro) map[string]interface{} {
	macroMap := make(map[string]interface{})
	macroMap["hash"] = macro.hash
	macroMap["hashKind"] = macro.hashKind
	macroMap["year"] = macro.year
	macroMap["month"] = macro.month
	macroMap["day"] = macro.day
	macroMap["hour"] = macro.hour
	macroMap["minute"] = macro.minute
	macroMap["second"] = macro.second
	macroMap["displayNum"] = macro.displayNum
	macroMap["AlphaMessage"] = macro.AlphaMessage

	return macroMap
}

func convert_macro_map_to_json(macroMap map[string]string) []byte {
	macroJSON, _ := json.Marshal(macroMap)
	return macroJSON
}

func wirte_macro_to_file(filePath string, fileName string, img *image.RGBA) {
	macro := init_macro(fileName, img)
	macromap := convert_macro_to_map(macro)
	macroJSON := convert_macro_map_to_json(macromap)

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		fmt.Println(err)
	}
	ti := exif.NewTagIndex()
	ib := exif.NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.TestDefaultByteOrder)
	err = ib.AddStandardWithName("DocumentName", string(macroJSON))
	if err != nil {
		fmt.Println(err)
	}

	intfc, _ := pis.NewPngMediaParser().ParseFile(filePath)
	cs := intfc.(*pis.ChunkSlice)
	err = cs.SetExif(ib)
	if err != nil {
		fmt.Println(err)
	}
	f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println(err)
	}
	err = cs.WriteTo(f)
	if err != nil {
		fmt.Println(err)
	}
	err = f.Close()
	if err != nil {
		fmt.Println(err)
	}

}

func convert_json_to_macro_map(jsonData string) map[string]string {
	var macroMap map[string]string
	json.Unmarshal([]byte(jsonData), &macroMap)
	return macroMap
}

func convert_map_to_macro(macroMap map[string]string) ImageMacro {
	macro := ImageMacro{}
	macro.hash, _ = strconv.ParseUint(macroMap["hash"], 10, 64)
	macro.hashKind = macroMap["hashKind"]
	macro.year, _ = strconv.Atoi(macroMap["year"])
	macro.month, _ = strconv.Atoi(macroMap["month"])
	macro.day, _ = strconv.Atoi(macroMap["day"])
	macro.hour, _ = strconv.Atoi(macroMap["hour"])
	macro.minute, _ = strconv.Atoi(macroMap["minute"])
	macro.second, _ = strconv.Atoi(macroMap["second"])
	macro.displayNum, _ = strconv.Atoi(macroMap["displayNum"])
	macro.AlphaMessage = macroMap["AlphaMessage"]

	return macro
}

func substract_macro_from_file(filePath string) (ImageMacro, error) {
	macro_emp := ImageMacro{}
	rawExif, err := exif.SearchFileAndExtractExif(filePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return macro_emp, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return macro_emp, err
	}

	ti := exif.NewTagIndex()

	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return macro_emp, err
	}

	tagName := "DocumentName"
	rootIfd := index.RootIfd

	// We know the tag we want is on IFD0 (the first/root IFD).
	results, err := rootIfd.FindTagWithName(tagName)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return macro_emp, err
	}

	ite := results[0]

	valueRaw, err := ite.Value()
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return macro_emp, err
	}
	value := valueRaw.(string)
	macroMap := convert_json_to_macro_map(value)
	macro := convert_map_to_macro(macroMap)
	return macro, nil
}
