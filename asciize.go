/*
MIT License

Copyright (c) 2022 epiccakeking

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"

	"golang.org/x/image/math/fixed"
)

var usedFont *truetype.Font

// Detect the line that best suits the region src.
func detLine(src *image.Gray) string {
	result := make([]byte, 0)
	myFace := truetype.NewFace(usedFont, &truetype.Options{})
	img := image.NewGray(src.Bounds())
	drawer := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{color.Black},
		Face: myFace,
		Dot:  fixed.Point26_6{X: 0, Y: (fixed.Int26_6(src.Bounds().Max.Y) << 6) * 2 / 3},
	}
	var x fixed.Int26_6 = 0
	maxX := fixed.Int26_6(src.Bounds().Max.X) << 6
	var i byte
	for x < maxX {
		var bestScore = 2 << 30
		var best byte
		newX := x
		// Skip control characters
		for i = 32; i < 127; i++ {
			draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
			drawer.Dot.X = x
			drawer.DrawBytes([]byte{i})
			// Skip if no glyph was rendered
			if drawer.Dot.X == x {
				continue
			}
			// Calculate the score (difference from desired image)
			score := 0
			for pixelX := int(x >> 6); pixelX < int(drawer.Dot.X>>6); pixelX++ {
				for pixelY := src.Bounds().Min.Y; pixelY < src.Bounds().Max.Y; pixelY++ {
					delta := int(src.GrayAt(pixelX, pixelY).Y) - int(img.GrayAt(pixelX, pixelY).Y)
					if delta < 0 {
						score -= delta
					} else {
						score += delta
					}
				}
			}
			// Normalize score based on width of the glyph
			score /= int((drawer.Dot.X - x) >> 6)
			if score < bestScore {
				bestScore = score
				best = i
				newX = drawer.Dot.X
			}
		}
		result = append(result, best)
		x = newX
	}
	return string(result)
}
func epanic(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	const LINE_HEIGHT = 20
	if len(os.Args) < 3 {
		fmt.Println("Usage:", os.Args[0], "FILE", "FONT")
		os.Exit(0)
	}
	// Decode font
	f, e := os.ReadFile(os.Args[2])

	epanic(e)
	font, err := freetype.ParseFont(f)
	if err != nil {
		panic(err)
	}

	usedFont = font
	// Decode image
	src, e := os.Open(os.Args[1])
	epanic(e)
	decoded, _, e := image.Decode(src)
	epanic(e)
	// Size of the image in ascii characters
	// Note: The program is currently hardcoded to a 8x16 font size
	size := decoded.Bounds().Size()
	size.Y /= LINE_HEIGHT
	asciinated := make([]string, size.Y)
	mut := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(size.Y)
	for y := range asciinated {
		go func(y int) {
			defer wg.Done()
			region := image.NewGray(image.Rect(0, 0, size.X, 16))
			draw.Draw(region, region.Bounds(), decoded, image.Point{X: 0, Y: y * LINE_HEIGHT}, draw.Src)
			result := detLine(region)
			mut.Lock()
			asciinated[y] = result
			mut.Unlock()
		}(y)
	}
	wg.Wait()
	for _, line := range asciinated {
		fmt.Println(strings.ReplaceAll(line, " ", "\u00A0"))
	}

}
