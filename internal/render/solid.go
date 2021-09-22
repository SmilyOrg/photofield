package render

import (
	"image/color"

	"github.com/tdewolff/canvas"
)

type Solid struct {
	Sprite Sprite
	Color  color.Color
}

func NewSolidFromRect(rect Rect, color color.Color) Solid {
	solid := Solid{}
	solid.Color = color
	solid.Sprite.Rect = rect
	return solid
}

func (solid *Solid) Draw(c *canvas.Context, scales Scales) {
	if solid.Sprite.IsVisible(c, scales) {
		prevFill := c.FillColor
		c.SetFillColor(solid.Color)
		solid.Sprite.Draw(c)
		c.SetFillColor(prevFill)
	}
}
