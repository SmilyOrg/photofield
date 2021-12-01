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
		image, _, err := source.GetImage(bitmap.Path)
		if err != nil {
			return err
		}

		bitmap.DrawImage(rimg, image, c)
	}
	return nil
}

func (bitmap *Bitmap) DrawImage(rimg draw.Image, img goimage.Image, c *canvas.Context) {
	bounds := img.Bounds()
	model := bitmap.Sprite.Rect.GetMatrixFitBoundsRotate(bounds, bitmap.Orientation)
	// modelTopLeft := model.Dot(canvas.Point{X: 0, Y: 0})
	// modelBottomRight := model.Dot(canvas.Point{X: float64(bounds.Max.X), Y: float64(bounds.Max.Y)})
	m := c.View().Mul(model)
	renderImageFast(rimg, img, m)
	// renderImageFastCropped(rimg, img, m, bitmap.Sprite.Rect, modelTopLeft, modelBottomRight)
}

func renderImageFast(rimg draw.Image, img goimage.Image, m canvas.Matrix) {
	bounds := img.Bounds()
	origin := m.Dot(canvas.Point{X: 0, Y: float64(bounds.Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{
		m[0][0], -m[0][1], origin.X,
		-m[1][0], m[1][1], h - origin.Y,
	}
	draw.ApproxBiLinear.Transform(rimg, aff3, img, bounds, draw.Src, nil)
}

// TODO finish implementation
func renderImageFastCropped(rimg draw.Image, img goimage.Image, m canvas.Matrix, crop Rect, modelTopLeft canvas.Point, modelBottomRight canvas.Point) {
	bounds := img.Bounds()
	// bounds := goimage.Rect(0, 0, int(modelBounds.X), int(modelBounds.Y))
	origin := m.Dot(canvas.Point{X: 0, Y: float64(bounds.Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{
		m[0][0], -m[0][1], origin.X,
		-m[1][0], m[1][1], h - origin.Y,
	}
	// croptl := m.Dot(canvas.Point{crop.X, crop.Y})
	// cropbr := m.Dot(canvas.Point{crop.X + crop.W, crop.Y + crop.H})
	// println(bounds.String(), crop.String(), croptl.String(), cropbr.String())
	// tx, ty := m.D

	model := Rect{
		X: modelTopLeft.X,
		Y: modelTopLeft.Y,
		W: modelBottomRight.X - modelTopLeft.X,
		H: modelBottomRight.Y - modelTopLeft.Y,
	}

	println(bounds.String(), "crop", crop.String(), "model", model.String())
	// bounds = bounds.Inset(10)
	// bounds =
	draw.ApproxBiLinear.Transform(rimg, aff3, img, bounds, draw.Src, nil)
}

func (bitmap *Bitmap) GetSize(source *image.Source) image.Size {
	info := source.GetInfo(bitmap.Path)
	return image.Size{X: info.Width, Y: info.Height}
}

func (bitmap *Bitmap) DrawOverdraw(c *canvas.Context, size goimage.Point) {
	style := c.Style

	pixelZoom := bitmap.Sprite.Rect.GetPixelZoom(c, size)
	// barWidth := -pixelZoom * 0.1
	// barHeight := 0.04
	alpha := pixelZoom * 0.025 * 0xFF
	max := 0.8 * float64(0xFF)
	if alpha < 0 {
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
