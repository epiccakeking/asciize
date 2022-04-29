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
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"

	"golang.org/x/image/math/fixed"
)

// Detect the character that best suits the region src.
func detChar(src *image.Gray) byte {
	img := image.NewGray(image.Rect(0, 0, 8, 16))
	var bestScore = 255*8*16 + 1 // Maximum possible value + 1
	var best byte
	var i byte
	drawer := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{color.Black},
		Face: inconsolata.Regular8x16,
		Dot:  fixed.Point26_6{X: 0, Y: 16 << 6},
	}
	for i = 0; i < 128; i++ {
		draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		drawer.Dot.X = 0
		drawer.DrawBytes([]byte{i})
		// Skip if no glyph was rendered
		if drawer.Dot.X == 0 {
			continue
		}
		// Calculate the score (difference from desired image)
		// This works on the assumption that the two images are the same dimensions
		score := 0
		for pixel := range img.Pix {
			delta := int(img.Pix[pixel]) - int(src.Pix[pixel])
			if delta < 0 {
				score -= delta
			} else {
				score += delta
			}
		}
		if score < bestScore {
			bestScore = score
			best = i
		}
	}
	return best
}
func epanic(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Usage:", os.Args[0], "FILE")
		os.Exit(0)
	}
	src, e := os.Open(os.Args[1])
	epanic(e)
	decoded, _, e := image.Decode(src)
	epanic(e)
	// Size of the image in ascii characters
	// Note: The program is currently hardcoded to a 8x16 font size
	size := decoded.Bounds().Size()
	size.X /= 8
	size.Y /= 16
	asciinated := make([][]byte, size.Y)
	for i := range asciinated {
		asciinated[i] = make([]byte, size.X)
	}
	mut := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(size.X * size.Y)
	for y := range asciinated {
		for x := range asciinated[y] {
			go func(x, y int) {
				defer wg.Done()
				region := image.NewGray(image.Rect(0, 0, 8, 16))
				draw.Draw(region, region.Bounds(), decoded, image.Point{X: x * 8, Y: y * 16}, draw.Src)
				result := detChar(region)
				mut.Lock()
				asciinated[y][x] = result
				mut.Unlock()
			}(x, y)
		}
	}
	wg.Wait()
	for _, line := range asciinated {
		fmt.Println(string(line))
	}

}
