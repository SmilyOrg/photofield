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

func fitInside(cw float64, ch float64, w float64, h float64) (float64, float64) {
	r := w / h
	cr := cw / ch
	if r < cr {
		h = w / cr
	} else {
		w = h * cr
	}
	return w, h
}

// func fitCenterInside(c Rect, r Rect) Rect {
// 	ar := r.W / r.H
// 	car := c.W / c.H
// 	if ar < car {
// 		h := r.W / car
// 		r.Y = h
// 	} else {
// 		r.W = r.H * car
// 	}
// 	return r
// }

func cropsBlackbarsOnly(img goimage.Image, crop goimage.Rectangle) bool {
	bounds := img.Bounds()

	sum := uint64(0)
	maxBlack := uint64(0xC00)

	// Horizontal top left black bar line
	for x := 0; x < crop.Min.X; x++ {
		c := img.At(x, bounds.Min.Y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("horizontal top left black bar line: %v\n", sum)

	// Horizontal top right black bar line
	for x := crop.Max.X; x < bounds.Max.X; x++ {
		c := img.At(x, bounds.Min.Y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("horizontal top right black bar line: %v\n", sum)

	// Horizontal bottom left black bar line
	for x := 0; x < crop.Min.X; x++ {
		c := img.At(x, bounds.Max.Y-1)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("horizontal bottom left black bar line: %v\n", sum)

	// Horizontal bottom right black bar line
	for x := crop.Max.X; x < bounds.Max.X; x++ {
		c := img.At(x, bounds.Max.Y-1)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("horizontal bottom right black bar line: %v\n", sum)

	// Vertical top left black bar line
	for y := 0; y < crop.Min.Y; y++ {
		c := img.At(bounds.Min.X, y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("vertical top left black bar line: %v\n", sum)

	// Vertical top right black bar line
	for y := crop.Max.Y; y < bounds.Max.Y; y++ {
		c := img.At(bounds.Min.X, y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("vertical top right black bar line: %v\n", sum)

	// Vertical bottom left black bar line
	for y := 0; y < crop.Min.Y; y++ {
		c := img.At(bounds.Max.X-1, y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("vertical bottom left black bar line: %v\n", sum)

	// Vertical bottom right black bar line
	for y := crop.Max.Y; y < bounds.Max.Y; y++ {
		c := img.At(bounds.Max.X-1, y)
		r, g, b, _ := c.RGBA()
		sum += uint64((r + g + b)) / 3 / maxBlack
	}
	// fmt.Printf("vertical bottom right black bar line: %v\n", sum)

	return sum < 2
}

func cropRect(bitmap *Bitmap, bounds goimage.Rectangle) goimage.Rectangle {
	brect := Rect{W: float64(bounds.Dx()), H: float64(bounds.Dy())}
	rect := bitmap.Sprite.Rect
	if bitmap.Orientation.SwapsDimensions() {
		rect.W, rect.H = rect.H, rect.W
	}
	cropr := rect.FitInside(brect)
	croprect := goimage.Rectangle{
		Min: goimage.Point{X: int(math.Round(cropr.X)), Y: int(math.Round(cropr.Y))},
		Max: goimage.Point{X: int(math.Round(cropr.X + cropr.W)), Y: int(math.Round(cropr.Y + cropr.H))},
	}
	return croprect
}

func (bitmap *Bitmap) DrawImage(rimg draw.Image, img goimage.Image, c *canvas.Context, scale float64) {
	bounds := img.Bounds()

	arb := float64(bounds.Dx()) / float64(bounds.Dy())
	aro := float64(bitmap.Sprite.Rect.W) / float64(bitmap.Sprite.Rect.H)
	ard := math.Abs(arb - aro)
	crop := ard > 0.05
	// cut = false

	var croprect goimage.Rectangle
	if crop {
		croprect = cropRect(bitmap, bounds)
		if !cropsBlackbarsOnly(img, croprect) {
			crop = false
		}
	}

	var model canvas.Matrix
	if crop {
		model = bitmap.Sprite.Rect.GetMatrixFillBoundsRotate(bounds, bitmap.Orientation)
	} else {
		model = bitmap.Sprite.Rect.GetMatrixFitBoundsRotate(bounds, bitmap.Orientation)
	}

	m := c.View().Mul(model.ScaleAbout(scale, scale, float64(bounds.Max.X)*0.5, float64(bounds.Max.Y)*0.5))
	if crop {
		renderImageFastCropped(rimg, img, m, croprect)
	} else {
		renderImageFast(rimg, img, m)
	}
}

func renderImageFastCropped(rimg draw.Image, img goimage.Image, m canvas.Matrix, crop goimage.Rectangle) {
	bounds := img.Bounds()
	origin := m.Dot(canvas.Point{X: 0, Y: float64(bounds.Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{
		m[0][0], -m[0][1], origin.X,
		-m[1][0], m[1][1], h - origin.Y,
	}
	draw.ApproxBiLinear.Transform(rimg, aff3, img, crop, draw.Src, nil)
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

func renderImageFastBounds(rimg draw.Image, img goimage.Image, m canvas.Matrix, bounds goimage.Rectangle) {
	origin := m.Dot(canvas.Point{X: 0, Y: float64(bounds.Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{
		m[0][0], -m[0][1], origin.X,
		-m[1][0], m[1][1], h - origin.Y,
	}
	draw.ApproxBiLinear.Transform(rimg, aff3, img, bounds, draw.Src, nil)
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
