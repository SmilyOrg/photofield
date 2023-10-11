package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	// "gioui.org/widget/material"

	playout "photofield/internal/layout"
	"photofield/internal/render"
	"photofield/internal/scene"
)

func runApp() {
	sceneConfig := defaultSceneConfig
	sceneConfig.Scene.Id = "Tqcqtc6h69"
	sceneConfig.Layout.ViewportWidth = 1280
	sceneConfig.Layout.ViewportHeight = 720
	sceneConfig.Layout.ImageHeight = 100
	sceneConfig.Layout.Type = playout.Map
	sceneConfig.Layout.Order = playout.DateAsc
	sceneConfig.Collection = *getCollectionById("ruhr-1000")

	go func() {
		w := app.NewWindow(
			app.Size(unit.Dp(sceneConfig.Layout.ViewportWidth), unit.Dp(sceneConfig.Layout.ViewportHeight)),
		)
		err := runMap(sceneConfig, w)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

var btag = new(bool) // We could use &pressed for this instead.
var pressed = false

func doButton(ops *op.Ops, q event.Queue) {
	// Process events that arrived between the last frame and this one.
	for _, ev := range q.Events(btag) {
		if x, ok := ev.(pointer.Event); ok {
			switch x.Type {
			case pointer.Press:
				pressed = true
			case pointer.Release:
				pressed = false
			}
		}
	}

	// Confine the area of interest to a 100x100 rectangle.
	defer clip.Rect{Max: image.Pt(100, 100)}.Push(ops).Pop()

	// Declare the tag.
	pointer.InputOp{
		Tag:   btag,
		Types: pointer.Press | pointer.Release,
	}.Add(ops)

	var c color.NRGBA
	if pressed {
		c = color.NRGBA{R: 0xFF, A: 0xFF}
	} else {
		c = color.NRGBA{G: 0xFF, A: 0xFF}
	}
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

func runMap(sceneConfig scene.SceneConfig, w *app.Window) error {
	// scene := sceneSource.Add(sceneConfig, imageSource)

	// click := gesture.Click{}
	// drag := gesture.Drag{}
	// scroll := gesture.Scroll{}

	// scrollTag := new(bool)

	// dragAnchor := f32.Pt(0, 0)
	// offset := f32.Pt(0, 0)
	// zoom := float32(1.)

	// th := material.NewTheme()
	var ops op.Ops
	for {
		e := <-w.Events()
		// click.Add(&ops)

		// println("pressed", click.Pressed())

		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)

			// layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// 	c := gio.NewContain(gtx, 200.0, 100.0)
			// 	ctx := canvas.NewContext(c)
			// 	if err := canvas.DrawPreview(ctx); err != nil {
			// 		panic(err)
			// 	}
			// 	return c.Dimensions()
			// })

			// click.Events(e.Queue)
			// click.Add(gtx.Ops)

			// devents := drag.Events(e.Metric, e.Queue, gesture.Both)
			// for _, d := range devents {
			// 	p := d.Position.Mul(1. / zoom)
			// 	switch {
			// 	case d.Type&pointer.Press != 0:
			// 		dragAnchor = p.Sub(offset)
			// 	case d.Type&pointer.Drag != 0:
			// 		offset = p.Sub(dragAnchor)
			// 	}
			// }
			// drag.Add(gtx.Ops)

			// scrollOffset := scroll.Scroll(e.Metric, e.Queue, time.Time{}, gesture.Vertical)
			// scroll.Add(gtx.Ops, image.Rect(-1000, -1000, 1000, 1000))

			// // ScrollBounds: image.Rect(0, 0, -),
			// // bounds should be int max
			// // newFunction(e, scrollTag, gtx)

			// zoom *= 1 - float32(scrollOffset)*0.001

			// println(click.Pressed())

			// doButton(&ops, e.Queue)

			// Zoom into the right side of the window
			// op.Affine(f32.NewAffine2D(
			// 	zoom, 0, 0,
			// 	0, zoom, 0,
			// )).Add(gtx.Ops)

			// op.Offset(image.Pt(int(offset.X), int(offset.Y))).Add(gtx.Ops)

			// drawScenePhotoRects(scene, &ops)

			e.Frame(gtx.Ops)
		}
	}
}

// func newFunction(q event.Queue) {
// 	for _, ev := range q.Events(scrollTag) {
// 		if x, ok := ev.(pointer.Event); ok {
// 			switch x.Type {
// 			case pointer.Scroll:
// 				println("scroll", x.Scroll.X, x.Scroll.Y)
// 			}
// 		}
// 	}
// 	pointer.InputOp{
// 		Tag:   scrollTag,
// 		Types: pointer.Scroll,

// 		ScrollBounds: image.Rect(-1000, -1000, 1000, 1000),
// 	}.Add(gtx.Ops)
// }

func drawScenePhotoRects(scene *render.Scene, ops *op.Ops) {
	op.Affine(f32.NewAffine2D(
		1, 0, 0,
		0, -1, float32(scene.Bounds.H),
	)).Add(ops)

	for _, photo := range scene.Photos {
		r := photo.Sprite.Rect
		drawRect(r, ops)
	}
}

func drawRect(r render.Rect, ops *op.Ops) {
	op.Affine(
		f32.Affine2D{}.
			Scale(f32.Pt(0, 0), f32.Pt(float32(r.W), float32(r.H))).
			Offset(f32.Pt(float32(r.X), float32(r.Y))),
		// Scale(f32.Pt(float32(-r.X), float32(-r.Y)), f32.Pt(float32(r.W), float32(r.H))),
	).Add(ops)

	defer clip.Rect{
		Min: image.Pt(0, 0),
		Max: image.Pt(10, 10),
	}.Push(ops).Pop()
	// fmt.Printf("x: %d, y: %d, w: %d, h: %d\n", int(r.X), int(r.Y), int(r.W), int(r.H))
	paint.ColorOp{Color: color.NRGBA{R: 0xFF, A: 0xFF}}.Add(ops)
	paint.PaintOp{}.Add(ops)
}
