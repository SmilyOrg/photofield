package render

import "github.com/tdewolff/canvas"

type Text struct {
	Sprite Sprite
	Font   *canvas.FontFace
	Text   string
}

func NewTextFromRect(rect Rect, font *canvas.FontFace, txt string) Text {
	text := Text{}
	text.Text = txt
	text.Font = font
	text.Sprite.Rect = rect
	return text
}

func (text *Text) Draw(c *canvas.Context, scales Scales) {
	if text.Sprite.IsVisible(c, scales) {
		textLine := canvas.NewTextLine(*text.Font, text.Text, canvas.Left)
		c.RenderText(textLine, c.View().Mul(text.Sprite.Rect.GetMatrix()))
	}
}
