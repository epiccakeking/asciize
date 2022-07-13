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

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

const (
	scoreShape = iota
	scoreShade
)

// Detect the line that best suits the region src.
func detLine(usedFont *sfnt.Font, src *image.Gray, progress chan fixed.Int26_6) string {
	result := make([]byte, 0)
	myFace, e := opentype.NewFace(usedFont, faceOptions)
	if e != nil {
		panic(e)
	}
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
			// Don't include out of bounds image data.
			rightBound := drawer.Dot.X.Ceil()
			if rightBound > maxX.Ceil() {
				rightBound = maxX.Ceil()
			}
			switch scoreMode {
			case scoreShape:
				for pixelX := x.Floor(); pixelX <= rightBound; pixelX++ {
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
				score /= rightBound - x.Floor() + 1
			case scoreShade:
				for pixelX := x.Floor(); pixelX <= rightBound; pixelX++ {
					for pixelY := src.Bounds().Min.Y; pixelY < src.Bounds().Max.Y; pixelY++ {
						score += int(src.GrayAt(pixelX, pixelY).Y) - int(img.GrayAt(pixelX, pixelY).Y)
					}
				}
				if score < 0 {
					score = -score
				}
				score /= rightBound - x.Floor() + 1
			}

			if score < bestScore {
				bestScore = score
				best = i
				newX = drawer.Dot.X
			}
			// Blank area used
			draw.Draw(img, image.Rect(x.Floor(), src.Bounds().Min.Y, drawer.Dot.X.Ceil(), src.Bounds().Max.Y), &image.Uniform{color.White}, image.Point{}, draw.Src)
		}
		if newX >= maxX {
			if showProgress {
				progress <- maxX - x
			}
			break
		}
		result = append(result, best)
		if showProgress {
			progress <- newX - x
		}
		x = newX
	}
	return string(result)
}

// Flags
var showProgress, useNbsp, trim bool
var fontPath string
var faceOptions = &opentype.FaceOptions{
	Size:    12,
	DPI:     72,
	Hinting: font.HintingNone,
}
var scoreMode = scoreShade
var scoreModeArg string

func init() {
	flag.BoolVar(&showProgress, "progress", false, "print progress")
	flag.BoolVar(&useNbsp, "nbsp", false, "convert spaces to no break space")
	flag.BoolVar(&trim, "trim", false, "trim trailing whitespace")
	flag.StringVar(&fontPath, "font", "", "path to ttf font file to use (uses gomono if unset)")
	flag.Float64Var(&faceOptions.Size, "size", 12, "font size to use")
	flag.StringVar(&scoreModeArg, "score", "shape", "how to score [shape/shade]")
}

func main() {
	flag.Parse()
	args := flag.Args()
	switch scoreModeArg {
	case "shape":
		scoreMode = scoreShape
	case "shade":
		scoreMode = scoreShade
	default: // Invalid arguments
		fmt.Fprintf(os.Stderr, "Bad scoring mode \"%s\"\n", scoreModeArg)
		flag.Usage()
		os.Exit(64)
	}

	if l := len(args); l != 1 {
		if l == 0 {
			fmt.Fprintln(os.Stderr, "No image file specified")
		} else {
			fmt.Fprintf(os.Stderr, "%d image files specified, expected 1\n", l)
		}
		flag.Usage()
		os.Exit(64)
	}

	// Decode font
	var usedFont *sfnt.Font
	if len(fontPath) > 0 {
		f, e := os.ReadFile(fontPath)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Failed to open font %s\n", fontPath)
			os.Exit(66)
		}
		usedFont, e = opentype.Parse(f)
		if e != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse font %s\n", fontPath)
			os.Exit(66)
		}
	} else {
		usedFont, _ = opentype.Parse(gomono.TTF)
	}

	// Create font face
	myFace, e := opentype.NewFace(usedFont, faceOptions)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse font %s\n", fontPath)
		os.Exit(66)
	}
	lineHeight := myFace.Metrics().Height.Ceil()

	// Decode image
	src, e := os.Open(args[0])
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to open image %s\n", args[0])
		os.Exit(66)
	}
	decoded, _, e := image.Decode(src)
	if e != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode image %s\n", args[0])
		os.Exit(66)
	}

	size := decoded.Bounds().Size()
	asciinated := make([]string, size.Y/lineHeight)
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
			region := image.NewGray(image.Rect(0, 0, size.X, lineHeight))
			draw.Draw(region, region.Bounds(), decoded, image.Point{X: 0, Y: y * lineHeight}, draw.Src)
			result := detLine(usedFont, region, progress)
			if trim {
				result = strings.TrimRight(result, " ")
			}
			if useNbsp {
				result = strings.ReplaceAll(result, " ", "\u00A0")
			}
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
		fmt.Println(line)
	}
}
