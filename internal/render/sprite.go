package render

import (
	"math"

	"github.com/tdewolff/canvas"
)

type Sprite struct {
	Rect Rect
}

func (sprite *Sprite) PlaceFitHeight(
	x float64,
	y float64,
	fitHeight float64,
	contentWidth float64,
	contentHeight float64,
) {
	scale := fitHeight / contentHeight
	if math.IsNaN(scale) || math.IsInf(scale, 0) {
		scale = 1
	}

	sprite.Rect = Rect{
		X: x,
		Y: y,
		W: contentWidth * scale,
		H: contentHeight * scale,
	}
}

func (sprite *Sprite) PlaceFit(
	x float64,
	y float64,
	fitWidth float64,
	fitHeight float64,
	contentWidth float64,
	contentHeight float64,
) {
	imageRatio := contentWidth / contentHeight

	var scale float64
	if fitWidth/fitHeight < imageRatio {
		scale = fitWidth / contentWidth
		// y = y - fitHeight*0.5 + scale*contentHeight*0.5
	} else {
		scale = fitHeight / contentHeight
		// x = x - width*0.5 + scale*contentWidth*0.5
	}

	sprite.Rect = Rect{
		X: x,
		Y: y,
		W: contentWidth * scale,
		H: contentHeight * scale,
	}
}

func (sprite *Sprite) Draw(c *canvas.Context) {
	c.RenderPath(
		canvas.Rectangle(sprite.Rect.W, sprite.Rect.H),
		c.Style,
		c.View().Mul(sprite.Rect.GetMatrix()),
	)
}

func (sprite *Sprite) DrawWithStyle(c *canvas.Context, style canvas.Style) {
	c.RenderPath(
		canvas.Rectangle(sprite.Rect.W, sprite.Rect.H),
		style,
		c.View().Mul(sprite.Rect.GetMatrix()),
	)
}

func (sprite *Sprite) DrawText(config *Render, c *canvas.Context, scales Scales, font *canvas.FontFace, txt string) {
	text := NewTextFromRect(sprite.Rect, font, txt)
	text.Draw(config, c, scales)
}

func (sprite *Sprite) IsVisible(c *canvas.Context, scales Scales) bool {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.Rect.W, H: sprite.Rect.H}
	canvasToUnit := canvas.Identity.
		Scale(scales.Tile, scales.Tile).
		Mul(c.View().Mul(sprite.Rect.GetMatrix()))
	unitRect := rect.Transform(canvasToUnit)
	return unitRect.X <= 1 && unitRect.Y <= 1 && unitRect.X+unitRect.W >= 0 && unitRect.Y+unitRect.H >= 0
}
