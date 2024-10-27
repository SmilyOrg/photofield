package render

import (
	"image/color"
	"photofield/internal/image"

	"github.com/tdewolff/canvas"
)

type Text struct {
	Sprite Sprite
	Font   *canvas.FontFace
	Text   string
	HAlign canvas.TextAlign
	VAlign canvas.TextAlign
}

func NewTextFromRect(rect Rect, font *canvas.FontFace, txt string) Text {
	text := Text{}
	text.Text = txt
	text.Font = font
	text.Sprite.Rect = rect
	return text
}

func (text *Text) Draw(config *Render, c *canvas.Context, scales Scales) {
	if text.Sprite.IsVisible(c, scales) {
		pixelArea := text.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})
		if pixelArea < config.MaxSolidPixelArea {
			// Skip rendering small text
			return
		}

		face := *text.Font
		face.Color = config.Color.(color.RGBA)

		textLine := canvas.NewTextBox(face, text.Text, text.Sprite.Rect.W, text.Sprite.Rect.H, text.HAlign, text.VAlign, 0, 0)
		rect := text.Sprite.Rect
		rect.Y -= rect.H
		c.RenderText(textLine, c.View().Mul(rect.GetMatrix()))
	}
}
