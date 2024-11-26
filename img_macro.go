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

type ImageMeta struct {
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

func init_Meta(fileName string, img *image.RGBA) ImageMeta {
	Meta := ImageMeta{}
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

	Meta.hash = imageHash
	Meta.hashKind = imageHashKind
	Meta.year = imageYear
	Meta.month = imageMonth
	Meta.day = imageDay
	Meta.hour = imageHour
	Meta.minute = imageMinute
	Meta.second = imageSecond
	Meta.displayNum = displayNum
	Meta.AlphaMessage = AlphaMessage

	return Meta
}

func convert_Meta_to_map(Meta ImageMeta) map[string]string {
	MetaMap := make(map[string]string)
	MetaMap["hash"] = fmt.Sprintf("%d", Meta.hash)
	MetaMap["hashKind"] = Meta.hashKind
	MetaMap["year"] = fmt.Sprintf("%d", Meta.year)
	MetaMap["month"] = fmt.Sprintf("%d", Meta.month)
	MetaMap["day"] = fmt.Sprintf("%d", Meta.day)
	MetaMap["hour"] = fmt.Sprintf("%d", Meta.hour)
	MetaMap["minute"] = fmt.Sprintf("%d", Meta.minute)
	MetaMap["second"] = fmt.Sprintf("%d", Meta.second)
	MetaMap["displayNum"] = fmt.Sprintf("%d", Meta.displayNum)
	MetaMap["AlphaMessage"] = Meta.AlphaMessage

	return MetaMap
}

func convert_Meta_to_interface_map(Meta ImageMeta) map[string]interface{} {
	MetaMap := make(map[string]interface{})
	MetaMap["hash"] = Meta.hash
	MetaMap["hashKind"] = Meta.hashKind
	MetaMap["year"] = Meta.year
	MetaMap["month"] = Meta.month
	MetaMap["day"] = Meta.day
	MetaMap["hour"] = Meta.hour
	MetaMap["minute"] = Meta.minute
	MetaMap["second"] = Meta.second
	MetaMap["displayNum"] = Meta.displayNum
	MetaMap["AlphaMessage"] = Meta.AlphaMessage

	return MetaMap
}

func convert_Meta_map_to_json(MetaMap map[string]string) []byte {
	MetaJSON, _ := json.Marshal(MetaMap)
	return MetaJSON
}

func wirte_Meta_to_file(filePath string, fileName string, img *image.RGBA) {
	Meta := init_Meta(fileName, img)
	Metamap := convert_Meta_to_map(Meta)
	MetaJSON := convert_Meta_map_to_json(Metamap)

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		fmt.Println(err)
	}
	ti := exif.NewTagIndex()
	ib := exif.NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.TestDefaultByteOrder)
	err = ib.AddStandardWithName("DocumentName", string(MetaJSON))
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

func convert_json_to_Meta_map(jsonData string) map[string]string {
	var MetaMap map[string]string
	json.Unmarshal([]byte(jsonData), &MetaMap)
	return MetaMap
}

func convert_map_to_Meta(MetaMap map[string]string) ImageMeta {
	Meta := ImageMeta{}
	Meta.hash, _ = strconv.ParseUint(MetaMap["hash"], 10, 64)
	Meta.hashKind = MetaMap["hashKind"]
	Meta.year, _ = strconv.Atoi(MetaMap["year"])
	Meta.month, _ = strconv.Atoi(MetaMap["month"])
	Meta.day, _ = strconv.Atoi(MetaMap["day"])
	Meta.hour, _ = strconv.Atoi(MetaMap["hour"])
	Meta.minute, _ = strconv.Atoi(MetaMap["minute"])
	Meta.second, _ = strconv.Atoi(MetaMap["second"])
	Meta.displayNum, _ = strconv.Atoi(MetaMap["displayNum"])
	Meta.AlphaMessage = MetaMap["AlphaMessage"]

	return Meta
}

func substract_Meta_from_file(filePath string) (ImageMeta, error) {
	Meta_emp := ImageMeta{}
	rawExif, err := exif.SearchFileAndExtractExif(filePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return Meta_emp, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return Meta_emp, err
	}

	ti := exif.NewTagIndex()

	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return Meta_emp, err
	}

	tagName := "DocumentName"
	rootIfd := index.RootIfd

	// We know the tag we want is on IFD0 (the first/root IFD).
	results, err := rootIfd.FindTagWithName(tagName)
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return Meta_emp, err
	}

	ite := results[0]

	valueRaw, err := ite.Value()
	if err != nil {
		fmt.Println(err)
		fmt.Println(filePath)
		return Meta_emp, err
	}
	value := valueRaw.(string)
	MetaMap := convert_json_to_Meta_map(value)
	Meta := convert_map_to_Meta(MetaMap)
	return Meta, nil
}
