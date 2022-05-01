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
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"

	_ "golang.org/x/image/webp"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"

	"golang.org/x/image/math/fixed"
)

// Detect the line that best suits the region src.
func detLine(usedFont *truetype.Font, src *image.Gray, progress chan fixed.Int26_6) string {
	result := make([]byte, 0)
	myFace := truetype.NewFace(usedFont, &truetype.Options{})
	img := image.NewGray(src.Bounds())
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	drawer := font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{color.Black},
		Face: myFace,
		Dot:  fixed.Point26_6{X: 0, Y: myFace.Metrics().Ascent},
	}
	var x fixed.Int26_6 = 0
	maxX := fixed.Int26_6(src.Bounds().Max.X) << 6
	var i byte
	for {
		var bestScore = 2 << 30
		var best byte
		newX := x
		// Skip control characters
		for i = 32; i < 127; i++ {
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
			// Blank area used
			draw.Draw(img, image.Rect(x.Floor(), src.Bounds().Min.Y, drawer.Dot.X.Ceil(), src.Bounds().Max.Y), &image.Uniform{color.White}, image.Point{}, draw.Src)
		}
		result = append(result, best)
		if x >= maxX {
			if showProgress {
				progress <- maxX - x
			}
			break
		}
		if showProgress {
			progress <- newX - x
		}
		x = newX
	}
	return string(result)
}
func epanic(e error) {
	if e != nil {
		panic(e)
	}
}

var showProgress, useNbsp, trim bool

func init() {
	flag.BoolVar(&showProgress, "progress", false, "print progress")
	flag.BoolVar(&useNbsp, "nbsp", false, "no break space")
	flag.BoolVar(&trim, "trim", false, "trim trailing whitespace")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n%s FILENAME FONT.TTF\n", os.Args[0], os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
	}
	// Decode font
	f, e := os.ReadFile(args[1])
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to read font %s\n", args[1])
		os.Exit(1)
	}
	usedFont, e := freetype.ParseFont(f)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse font %s\n", args[1])
		os.Exit(1)
	}

	LINE_HEIGHT := int(truetype.NewFace(usedFont, &truetype.Options{}).Metrics().Height >> 6)
	// Decode image
	src, e := os.Open(args[0])
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to open image %s\n", args[0])
		os.Exit(1)
	}
	decoded, _, e := image.Decode(src)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode image %s\n", args[0])
		os.Exit(1)
	}
	size := decoded.Bounds().Size()
	asciinated := make([]string, size.Y/LINE_HEIGHT)
	progress := make(chan fixed.Int26_6)
	mut := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(asciinated))
	// Close progress channel on completion
	go func() {
		wg.Wait()
		close(progress)
	}()
	for y := range asciinated {
		go func(y int) {
			defer wg.Done()
			region := image.NewGray(image.Rect(0, 0, size.X, LINE_HEIGHT))
			draw.Draw(region, region.Bounds(), decoded, image.Point{X: 0, Y: y * LINE_HEIGHT}, draw.Src)
			result := detLine(usedFont, region, progress)
			mut.Lock()
			asciinated[y] = result
			mut.Unlock()
		}(y)
	}
	total_progress := float64(len(asciinated) * size.X << 6)
	var current_progress float64 = 0
	for delta := range progress {
		current_progress += float64(delta)
		fmt.Fprintf(os.Stderr, "\rProgress: %%%.2f", current_progress*100/total_progress)
	}
	// Move to next line if any progress reports were made
	if current_progress > 0 {
		fmt.Fprintln(os.Stderr)
	}
	for _, line := range asciinated {
		if trim {
			line = strings.TrimRight(line, " ")
		}
		if useNbsp {
			line = strings.ReplaceAll(line, " ", "\u00A0")
		}
		fmt.Println(line)
	}

}
