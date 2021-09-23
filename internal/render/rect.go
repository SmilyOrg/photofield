package render

import (
	"fmt"
	goimage "image"
	"photofield/internal/image"

	"github.com/tdewolff/canvas"
)

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

func NewRectFromCanvasRect(r canvas.Rect) Rect {
	return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

func (rect Rect) ToCanvasRect() canvas.Rect {
	return canvas.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
}

func (rect Rect) Move(offset Point) Rect {
	rect.X += offset.X
	rect.Y += offset.Y
	return rect
}

func (rect Rect) ScalePoint(scale Point) Rect {
	rect.X *= scale.X
	rect.W *= scale.X
	rect.Y *= scale.Y
	rect.H *= scale.Y
	return rect
}

func (rect Rect) Scale(scale float64) Rect {
	rect.X *= scale
	rect.W *= scale
	rect.Y *= scale
	rect.H *= scale
	return rect
}

func (rect Rect) Transform(m canvas.Matrix) Rect {
	return NewRectFromCanvasRect(rect.ToCanvasRect().Transform(m))
}

func (rect Rect) String() string {
	return fmt.Sprintf("%3.3f %3.3f %3.3f %3.3f", rect.X, rect.Y, rect.W, rect.H)
}

func (rect Rect) FitInside(container Rect) Rect {
	imageRatio := rect.W / rect.H

	var scale float64
	if container.W/container.H < imageRatio {
		scale = container.W / rect.W
	} else {
		scale = container.H / rect.H
	}

	return Rect{
		X: container.X,
		Y: container.Y,
		W: rect.W * scale,
		H: rect.H * scale,
	}
}

func (rect Rect) GetMatrix() canvas.Matrix {
	return canvas.Identity.
		Translate(rect.X, -rect.Y-rect.H)
}

func (rect Rect) GetMatrixFitWidth(width float64) canvas.Matrix {
	scale := rect.W / width
	return rect.GetMatrix().
		Scale(scale, scale)
}

func (rect Rect) GetMatrixFitInside(width float64, height float64) canvas.Matrix {
	ratio := rect.W / rect.H

	matrix := rect.GetMatrix()

	if width/height > ratio {
		scale := rect.W / width
		matrix = matrix.Translate(0, (rect.H-height*scale)*0.5).Scale(scale, scale)
	} else {
		scale := rect.H / height
		matrix = matrix.Translate((rect.W-width*scale)*0.5, 0).Scale(scale, scale)
	}

	return matrix
}

func (rect Rect) GetMatrixFitImage(image *goimage.Image) canvas.Matrix {
	bounds := (*image).Bounds()
	return rect.GetMatrixFitWidth(float64(bounds.Max.X) - float64(bounds.Min.X))
}

func (rect Rect) GetMatrixFitImageRotate(img *goimage.Image, orientation image.Orientation) canvas.Matrix {
	bounds := (*img).Bounds()
	imageWidth := float64(bounds.Max.X - bounds.Min.X)
	imageHeight := float64(bounds.Max.Y - bounds.Min.Y)

	if orientation.SwapsDimensions() {
		imageWidth, imageHeight = imageHeight, imageWidth
	}

	matrix := rect.GetMatrixFitInside(imageWidth, imageHeight)
	switch orientation {
	case image.MirrorHorizontal:
		matrix = matrix.Translate(imageWidth, 0).ReflectX()

	case image.Rotate180:
		matrix = matrix.Translate(imageWidth, imageHeight).Rotate(-180)

	case image.MirrorVertical:
		matrix = matrix.Translate(0, imageHeight).ReflectY()

	case image.MirrorHorizontalRotate270:
		matrix = matrix.Translate(imageWidth, imageHeight).Rotate(-270).ReflectX()

	case image.Rotate90:
		matrix = matrix.Translate(0, imageHeight).Rotate(-90)

	case image.MirrorHorizontalRotate90:
		matrix = matrix.Rotate(-90).ReflectX()

	case image.Rotate270:
		matrix = matrix.Translate(imageWidth, 0).Rotate(-270)
	}

	return matrix
}

func (rect Rect) IsVisible(view Rect) bool {
	return rect.X <= view.X+view.W &&
		rect.Y <= view.Y+view.H &&
		rect.X+rect.W >= view.X &&
		rect.Y+rect.H >= view.Y
}

func (rect Rect) GetPixelArea(c *canvas.Context, size image.Size) float64 {
	pixel := canvas.Rect{X: 0, Y: 0, W: 1, H: 1}
	canvasToTile := c.View().Mul(rect.GetMatrixFitWidth(float64(size.X)))
	tileRect := pixel.Transform(canvasToTile)
	// fmt.Printf("rect w %4.0f h %4.0f   size w %4.0f h %4.0f   tileRect w %4f h %4f\n", rect.W, rect.H, size.Width, size.Height, tileRect.W, tileRect.H)
	// tx, ty, theta, sx, sy, phi := canvasToTile.Decompose()
	// log.Printf("tx %f ty %f theta %f sx %f sy %f phi %f rectw %f tw %f th %f\n", tx, ty, theta, sx, sy, phi, rect.W, tileRect.W, tileRect.H)
	area := tileRect.W * tileRect.H
	return area
}

func (rect Rect) GetPixelZoom(c *canvas.Context, size image.Size) float64 {
	pixelArea := rect.GetPixelArea(c, size)
	if pixelArea >= 1 {
		return pixelArea
	} else {
		return -1 / pixelArea
	}
}

func (rect Rect) GetPixelZoomDist(c *canvas.Context, size image.Size) float64 {
	// return math.Abs(rect.GetPixelZoom(c, size))
	zoom := rect.GetPixelZoom(c, size)
	if zoom > 0 {
		return zoom * 3
	} else {
		return -zoom
	}
}
