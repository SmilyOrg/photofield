package render

import (
	goimage "image"
	"image/color"
	"math"
	"photofield/internal/image"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

func getRGBA(col color.Color) color.RGBA {
	r, g, b, a := col.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

type Bitmap struct {
	Path        string
	Sprite      Sprite
	Orientation image.Orientation
}

func (bitmap *Bitmap) Draw(rimg draw.Image, c *canvas.Context, scales Scales, source *image.Source) error {
	if bitmap.Sprite.IsVisible(c, scales) {
		image, err := source.GetImage(bitmap.Path)
		if err != nil {
			return err
		}

		model := bitmap.Sprite.Rect.GetMatrixFitImageRotate(image, bitmap.Orientation)
		m := c.View().Mul(model)

		renderImageFast(rimg, *image, m)
	}
	return nil
}

func renderImageFast(rimg draw.Image, img goimage.Image, m canvas.Matrix) {
	origin := m.Dot(canvas.Point{X: 0, Y: float64(img.Bounds().Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{m[0][0], -m[0][1], origin.X, -m[1][0], m[1][1], h - origin.Y}
	draw.ApproxBiLinear.Transform(rimg, aff3, img, img.Bounds(), draw.Src, nil)
}

func (bitmap *Bitmap) GetSize(source *image.Source) image.Size {
	info := source.GetInfo(bitmap.Path)
	return image.Size{X: info.Width, Y: info.Height}
}

func (bitmap *Bitmap) DrawOverdraw(c *canvas.Context, source *image.Source) {
	style := c.Style

	size := bitmap.GetSize(source)
	pixelZoom := bitmap.Sprite.Rect.GetPixelZoom(c, size)
	barWidth := -pixelZoom * 0.1
	// barHeight := 0.04
	alpha := pixelZoom * 0.1 * 0xFF
	max := 0.8 * float64(0xFF)
	if barWidth > 0 {
		alpha = math.Min(max, math.Max(0, -alpha))
		style.FillColor = getRGBA(color.NRGBA{0xFF, 0x00, 0x00, uint8(alpha)})
	} else {
		alpha = math.Min(max, math.Max(0, alpha))
		style.FillColor = getRGBA(color.NRGBA{0x00, 0x00, 0xFF, uint8(alpha)})
	}

	bitmap.Sprite.DrawWithStyle(c, style)

	// style.FillColor = canvas.Yellowgreen
	// c.RenderPath(
	// 	canvas.Rectangle(bitmap.Sprite.Rect.W*0.5*barWidth, bitmap.Sprite.Rect.H*barHeight),
	// 	style,
	// 	c.View().Mul(bitmap.Sprite.Rect.GetMatrix()).
	// 		Translate(
	// 			bitmap.Sprite.Rect.W*0.5,
	// 			bitmap.Sprite.Rect.H*(0.5-barHeight*0.5),
	// 		),
	// )
}

func (bitmap *Bitmap) DrawVideoIcon(c *canvas.Context) {
	style := c.Style

	sprite := bitmap.Sprite

	iconSize := sprite.Rect.H * 0.04
	marginTop := iconSize * 1.5
	marginRight := iconSize * 1.5

	style.FillColor = getRGBA(color.White)
	style.StrokeColor = getRGBA(color.RGBA{R: 0, G: 0, B: 0, A: 0xCC})

	canvasIconSize := canvas.Rect{W: iconSize}.Transform(c.View()).W

	style.StrokeWidth = canvasIconSize * 0.2
	style.StrokeJoiner = canvas.RoundJoiner{}

	c.RenderPath(
		canvas.RegularPolygon(3, iconSize, true),
		style,
		c.View().Mul(sprite.Rect.GetMatrix()).Translate(sprite.Rect.W-marginRight, sprite.Rect.H-marginTop).Rotate(30),
	)
}
