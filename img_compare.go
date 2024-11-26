package main

import (
	"errors"
	"image"

	"github.com/nfnt/resize"
)

type ImageHash struct {
	hash uint64
	kind string
}

func Rgb2Gray(colorImg image.Image) [][]float64 {
	bounds := colorImg.Bounds()
	w, h := bounds.Max.X-bounds.Min.X, bounds.Max.Y-bounds.Min.Y
	pixels := make([][]float64, h)

	for i := range pixels {
		pixels[i] = make([]float64, w)
		for j := range pixels[i] {
			color := colorImg.At(j, i)
			r, g, b, _ := color.RGBA()
			lum := 0.299*float64(r/257) + 0.587*float64(g/257) + 0.114*float64(b/256)
			pixels[i][j] = lum
		}
	}

	return pixels
}

func FlattenPixels(pixels [][]float64, x int, y int) []float64 {
	flattens := make([]float64, x*y)
	for i := 0; i < y; i++ {
		for j := 0; j < x; j++ {
			flattens[y*i+j] = pixels[i][j]
		}
	}
	return flattens
}

func MeanOfPixels(pixels []float64) float64 {
	m := 0.0
	lens := len(pixels)
	if lens == 0 {
		return 0
	}

	for _, p := range pixels {
		m += p
	}

	return m / float64(lens)
}

func (h *ImageHash) leftShiftSet(idx int) {
	h.hash |= 1 << uint(idx)
}

func popcnt(x uint64) int {
	diff := 0
	for x != 0 {
		diff += int(x & 1)
		x >>= 1
	}

	return diff
}

func (h *ImageHash) Distance(other *ImageHash) (int, error) {

	lhash := h.hash
	rhash := other.hash

	hamming := lhash ^ rhash
	return popcnt(hamming), nil
}

func AverageHash(img *image.RGBA) (*ImageHash, error) {
	if img == nil {
		return nil, errors.New("image object can not be nil")
	}

	// Create 64bits hash.
	ahash := &ImageHash{0, "ahash"}
	resized := resize.Resize(8, 8, img, resize.Bilinear)
	pixels := Rgb2Gray(resized)
	flattens := FlattenPixels(pixels, 8, 8)
	avg := MeanOfPixels(flattens)

	for idx, p := range flattens {
		if p > avg {
			ahash.leftShiftSet(len(flattens) - idx - 1)
		}
	}

	return ahash, nil
}

func img_distance(img1 *image.RGBA, img2 *image.RGBA) int {
	// convert *image.RGBA to image.Image
	// img1Image := image.Image(img1)
	// img2Image := image.Image(img2)
	hash1, _ := AverageHash(img1)
	hash2, _ := AverageHash(img2)
	distance, _ := hash1.Distance(hash2)
	return distance
}
