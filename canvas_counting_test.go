package blazon

import "image/color"

// countingCanvas records how many subpaths a renderer starts. It is enough to
// assert stroke counts without inspecting geometry.
type countingCanvas struct {
	moves int
}

func (c *countingCanvas) Size() (float64, float64)                      { return 100, 100 }
func (c *countingCanvas) Background(color.Color)                        {}
func (c *countingCanvas) MoveTo(x, y float64)                           { c.moves++ }
func (c *countingCanvas) LineTo(x, y float64)                           {}
func (c *countingCanvas) CubicTo(a, b, d, e, f, g float64)              {}
func (c *countingCanvas) Close()                                        {}
func (c *countingCanvas) Fill(color.Color)                              {}
func (c *countingCanvas) Stroke(color.Color, float64)                   {}
func (c *countingCanvas) SetGrid(cols, rows int)                        {}
func (c *countingCanvas) Cell(col, row int, r rune, fg, bg color.Color) {}
