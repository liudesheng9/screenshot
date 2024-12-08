package image_manipulation

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
	Hash         uint64
	HashKind     string
	Year         int
	Month        int
	Day          int
	Hour         int
	Minute       int
	Second       int
	DisplayNum   int
	AlphaMessage string
}

func Init_Meta(fileName string, img *image.RGBA) ImageMeta {
	Meta := ImageMeta{}
	hash, _ := AverageHash(img)
	imageHash := hash.Hash
	imageHashKind := hash.Kind
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

	Meta.Hash = imageHash
	Meta.HashKind = imageHashKind
	Meta.Year = imageYear
	Meta.Month = imageMonth
	Meta.Day = imageDay
	Meta.Hour = imageHour
	Meta.Minute = imageMinute
	Meta.Second = imageSecond
	Meta.DisplayNum = displayNum
	Meta.AlphaMessage = AlphaMessage

	return Meta
}

func convert_Meta_to_map(Meta ImageMeta) map[string]string {
	MetaMap := make(map[string]string)
	MetaMap["hash"] = fmt.Sprintf("%d", Meta.Hash)
	MetaMap["hashKind"] = Meta.HashKind
	MetaMap["year"] = fmt.Sprintf("%d", Meta.Year)
	MetaMap["month"] = fmt.Sprintf("%d", Meta.Month)
	MetaMap["day"] = fmt.Sprintf("%d", Meta.Day)
	MetaMap["hour"] = fmt.Sprintf("%d", Meta.Hour)
	MetaMap["minute"] = fmt.Sprintf("%d", Meta.Minute)
	MetaMap["second"] = fmt.Sprintf("%d", Meta.Second)
	MetaMap["displayNum"] = fmt.Sprintf("%d", Meta.DisplayNum)
	MetaMap["AlphaMessage"] = Meta.AlphaMessage

	return MetaMap
}

func Convert_Meta_to_interface_map(Meta ImageMeta) map[string]interface{} {
	MetaMap := make(map[string]interface{})
	MetaMap["hash"] = Meta.Hash
	MetaMap["hashKind"] = Meta.HashKind
	MetaMap["year"] = Meta.Year
	MetaMap["month"] = Meta.Month
	MetaMap["day"] = Meta.Day
	MetaMap["hour"] = Meta.Hour
	MetaMap["minute"] = Meta.Minute
	MetaMap["second"] = Meta.Second
	MetaMap["displayNum"] = Meta.DisplayNum
	MetaMap["AlphaMessage"] = Meta.AlphaMessage

	return MetaMap
}

func Convert_Meta_map_to_json(MetaMap map[string]string) []byte {
	MetaJSON, _ := json.Marshal(MetaMap)
	return MetaJSON
}

func Wirte_Meta_to_file(filePath string, fileName string, img *image.RGBA) {
	Meta := Init_Meta(fileName, img)
	Metamap := convert_Meta_to_map(Meta)
	MetaJSON := Convert_Meta_map_to_json(Metamap)

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

func Convert_json_to_Meta_map(jsonData string) map[string]string {
	var MetaMap map[string]string
	json.Unmarshal([]byte(jsonData), &MetaMap)
	return MetaMap
}

func Convert_map_to_Meta(MetaMap map[string]string) ImageMeta {
	Meta := ImageMeta{}
	Meta.Hash, _ = strconv.ParseUint(MetaMap["hash"], 10, 64)
	Meta.HashKind = MetaMap["hashKind"]
	Meta.Year, _ = strconv.Atoi(MetaMap["year"])
	Meta.Month, _ = strconv.Atoi(MetaMap["month"])
	Meta.Day, _ = strconv.Atoi(MetaMap["day"])
	Meta.Hour, _ = strconv.Atoi(MetaMap["hour"])
	Meta.Minute, _ = strconv.Atoi(MetaMap["minute"])
	Meta.Second, _ = strconv.Atoi(MetaMap["second"])
	Meta.DisplayNum, _ = strconv.Atoi(MetaMap["displayNum"])
	Meta.AlphaMessage = MetaMap["AlphaMessage"]

	return Meta
}

func Substract_Meta_from_file(filePath string) (ImageMeta, error) {
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
	MetaMap := Convert_json_to_Meta_map(value)
	Meta := Convert_map_to_Meta(MetaMap)
	return Meta, nil
}
