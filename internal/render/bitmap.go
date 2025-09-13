package render

import (
	"context"
	goimage "image"
	"image/color"
	"math"
	"photofield/internal/image"
	"runtime/trace"

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

func pixelColorDivSum(img goimage.Image, xmin int, xmax int, ymin int, ymax int) uint64 {
	sum := uint64(0)
	div := uint64(10 * 0x100 * 3)
	for y := ymin; y <= ymax; y++ {
		for x := xmin; x <= xmax; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			sum += (uint64(r) + uint64(g) + uint64(b)) / div
		}
	}
	return sum
}

func cropsBlackbarsOnly(img goimage.Image, crop goimage.Rectangle) bool {
	bounds := img.Bounds()
	bx1, bx2, by1, by2 := bounds.Min.X, bounds.Max.X, bounds.Min.Y, bounds.Max.Y
	cx1, cx2, cy1, cy2 := crop.Min.X, crop.Max.X, crop.Min.Y, crop.Max.Y

	m := 1 // margin
	sum := uint64(0)
	sum += pixelColorDivSum(img, bx1+0, cx1-1-m, cy1+0, cy1+0) // Top Left Horizontal
	sum += pixelColorDivSum(img, cx2+m, bx2-1, cy1+0, cy1+0)   // Top Right Horizontal
	sum += pixelColorDivSum(img, bx1+0, cx1-1-m, cy2-1, cy2-1) // Bottom Left Horizontal
	sum += pixelColorDivSum(img, cx2+m, bx2-1, cy2-1, cy2-1)   // Bottom Right Horizontal
	sum += pixelColorDivSum(img, cx1+0, cx1+0, by1+0, cy1-1-m) // Top Left Vertical
	sum += pixelColorDivSum(img, cx2-1, cx2-1, by1+0, cy1-1-m) // Top Right Vertical
	sum += pixelColorDivSum(img, cx1+0, cx1+0, cy2+m, by2-1)   // Bottom Left Vertical
	sum += pixelColorDivSum(img, cx2-1, cx2-1, cy2+m, by2-1)   // Bottom Right Vertical

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

func (bitmap *Bitmap) DrawImage(ctx context.Context, rimg draw.Image, img goimage.Image, c *canvas.Context, scale float64, hq bool) {
	defer trace.StartRegion(ctx, "bitmap.DrawImage").End()

	bounds := img.Bounds()

	arb := float64(bounds.Dx()) / float64(bounds.Dy())
	aro := float64(bitmap.Sprite.Rect.W) / float64(bitmap.Sprite.Rect.H)
	ard := math.Abs(arb - aro)
	crop := ard > 0.05
	// crop = false

	var croprect goimage.Rectangle
	if crop {
		croprect = cropRect(bitmap, bounds)
		if !cropsBlackbarsOnly(img, croprect) {
			crop = false
			// Cropping is disabled because the detected crop rectangle would remove more than just black bars.
			// Use the full image bounds as the crop rectangle in this fallback case.
			croprect = bounds
		}
	} else {
		croprect = bounds
	}

	var model canvas.Matrix
	if crop {
		model = bitmap.Sprite.Rect.GetMatrixFillBoundsRotate(bounds, bitmap.Orientation)
	} else {
		model = bitmap.Sprite.Rect.GetMatrixFitBoundsRotate(bounds, bitmap.Orientation)
	}

	m := c.View().Mul(model.ScaleAbout(scale, scale, float64(bounds.Max.X)*0.5, float64(bounds.Max.Y)*0.5))

	var interp draw.Interpolator
	if hq {
		interp = draw.CatmullRom
	} else {
		interp = draw.ApproxBiLinear
	}
	renderImage(ctx, rimg, img, m, croprect, interp)
}

func renderImage(ctx context.Context, rimg draw.Image, img goimage.Image, m canvas.Matrix, crop goimage.Rectangle, interpolator draw.Interpolator) {
	defer trace.StartRegion(ctx, "renderImage").End()

	bounds := img.Bounds()
	origin := m.Dot(canvas.Point{X: 0, Y: float64(bounds.Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{
		m[0][0], -m[0][1], origin.X,
		-m[1][0], m[1][1], h - origin.Y,
	}
	interpolator.Transform(rimg, aff3, img, crop, draw.Src, nil)
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
