package render

import (
	"github.com/tdewolff/canvas"
)

type Line struct {
	A     Point
	B     Point
	Width float64
}

func (line *Line) Draw(config *Render, c *canvas.Context, scales Scales) {
	p := canvas.Path{}
	p.MoveTo(line.A.X, -line.A.Y)
	p.LineTo(line.B.X, -line.B.Y)
	// p.MoveTo(-10000, -10000)
	// p.LineTo(10000, 10000)

	w := line.Width * scales.Pixel
	s := canvas.Style{
		StrokeWidth:  w,
		StrokeColor:  canvas.Darkgray,
		StrokeCapper: canvas.SquareCap,
		StrokeJoiner: canvas.RoundJoin,
		Dashes:       []float64{w * 2, w * 4},
		DashOffset:   0,
	}

	c.RenderPath(&p, s, c.View())
	// c.RenderPath(&p, s, canvas.Identity)

	// if text.Sprite.IsVisible(c, scales) {

	// pixelArea := text.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})
	// if pixelArea < config.MaxSolidPixelArea {
	// 	// Skip rendering small text
	// 	return
	// }

	// textLine := canvas.NewTextBox(*text.Font, text.Text, text.Sprite.Rect.W, text.Sprite.Rect.H, text.HAlign, text.VAlign, 0, 0)
	// rect := text.Sprite.Rect
	// rect.Y -= rect.H
	// c.RenderText(textLine, c.View().Mul(rect.GetMatrix()))
	// }
}
