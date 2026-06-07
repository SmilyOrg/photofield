package layout

import (
	"context"
	"photofield/internal/image"
	"photofield/internal/render"
	"strings"
)

type FacePhoto struct {
	FileId     image.ImageId
	FaceId     int
	X          int
	Y          int
	W          int
	H          int
	Confidence int
	Info       image.Info
}

type FaceRegionData struct {
	PhotoRegionData     // Embed standard photo data
	FaceId          int `json:"face_id"`
	BboxX           int `json:"bbox_x"`
	BboxY           int `json:"bbox_y"`
	BboxW           int `json:"bbox_w"`
	BboxH           int `json:"bbox_h"`
	Confidence      int `json:"confidence"`
}

type FaceRegionSource struct {
	Source *image.Source
	Faces  []FacePhoto // All faces in order
}

func (regionSource FaceRegionSource) getRegionFromFace(
	id int,
	facePhoto *FacePhoto,
	scene *render.Scene,
	regionConfig render.RegionConfig,
) render.Region {
	if id <= 0 || id > len(scene.Photos) {
		return render.Region{}
	}

	photo := &scene.Photos[id-1]

	// For minimal responses, skip the expensive photo data lookup
	if regionConfig.Minimal {
		return render.Region{
			Id:     id,
			Bounds: photo.Sprite.Rect,
		}
	}

	// Get standard photo region data
	photoRegion := PhotoRegionSource{Source: regionSource.Source}.getRegionFromPhoto(
		id, photo, scene, regionConfig,
	)

	// Extend with face-specific data
	photoData, _ := photoRegion.Data.(PhotoRegionData)
	return render.Region{
		Id:     id,
		Bounds: photo.Sprite.Rect,
		Data: FaceRegionData{
			PhotoRegionData: photoData,
			FaceId:          facePhoto.FaceId,
			BboxX:           facePhoto.X,
			BboxY:           facePhoto.Y,
			BboxW:           facePhoto.W,
			BboxH:           facePhoto.H,
			Confidence:      facePhoto.Confidence,
		},
	}
}

func (regionSource FaceRegionSource) GetRegionsFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	photos := scene.GetVisiblePhotoRefs(context.TODO(), rect, regionConfig.Limit)
	for photoRef := range photos {
		if photoRef.Index < len(regionSource.Faces) {
			facePhoto := regionSource.Faces[photoRef.Index]
			regions = append(regions, regionSource.getRegionFromFace(
				1+photoRef.Index,
				&facePhoto,
				scene,
				regionConfig,
			))
		}
	}
	return regions
}

func (regionSource FaceRegionSource) GetRegionsFromImageId(id image.ImageId, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	max := regionConfig.Limit
	if max == 0 {
		max = len(scene.Photos)
	}

	for i := range scene.Photos {
		photo := &scene.Photos[i]
		if photo.Id != id {
			continue
		}
		if i < len(regionSource.Faces) {
			facePhoto := regionSource.Faces[i]
			regions = append(regions, regionSource.getRegionFromFace(
				1+i,
				&facePhoto,
				scene,
				regionConfig,
			))
		}
		if len(regions) >= max {
			break
		}
	}
	return regions
}

func (regionSource FaceRegionSource) GetRegionChanFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) <-chan render.Region {
	out := make(chan render.Region)
	go func() {
		photos := scene.GetVisiblePhotoRefs(context.TODO(), rect, regionConfig.Limit)
		for photoRef := range photos {
			if photoRef.Index < len(regionSource.Faces) {
				facePhoto := regionSource.Faces[photoRef.Index]
				out <- regionSource.getRegionFromFace(
					1+photoRef.Index,
					&facePhoto,
					scene,
					regionConfig,
				)
			}
		}
		close(out)
	}()
	return out
}

func (regionSource FaceRegionSource) GetRegionById(id int, scene *render.Scene, regionConfig render.RegionConfig) render.Region {
	if id <= 0 || id > len(scene.Photos) {
		return render.Region{}
	}

	if id-1 < len(regionSource.Faces) {
		facePhoto := regionSource.Faces[id-1]
		return regionSource.getRegionFromFace(id, &facePhoto, scene, regionConfig)
	}

	return render.Region{}
}

func (regionSource FaceRegionSource) GetRegionClosestTo(p render.Point, scene *render.Scene, regionConfig render.RegionConfig) (region render.Region, ok bool) {
	photoRef, ok := scene.GetClosestPhotoRef(p)
	if !ok {
		return render.Region{}, false
	}

	if photoRef.Index < len(regionSource.Faces) {
		facePhoto := regionSource.Faces[photoRef.Index]
		return regionSource.getRegionFromFace(1+photoRef.Index, &facePhoto, scene, regionConfig), true
	}

	return render.Region{}, false
}

func LayoutFaces(faces <-chan FacePhoto, layout Layout, scene *render.Scene, source *image.Source) {
	spacing := 10.0
	padding := 20.0
	topMargin := 64.0
	faceSize := layout.ImageHeight
	faceBuffer := 1.2

	if strings.Contains(layout.Tweaks, "nomargin") {
		padding = 0
		topMargin = 0
	} else if strings.Contains(layout.Tweaks, "notopmargin") {
		topMargin = 0
	}

	layoutWidth := layout.ViewportWidth - 2*padding

	columns := int(layoutWidth / (faceSize + spacing))
	if columns < 1 {
		columns = 1
	}

	x := padding
	y := padding + topMargin

	// Collect all faces for region source
	var allFaces []FacePhoto

	for face := range faces {
		// Create a photo sprite for the face
		photo := render.Photo{
			Id: face.FileId,
		}

		// Set the sprite rectangle for the face crop area
		photo.Sprite.Rect = render.Rect{
			X: x,
			Y: y,
			W: faceSize,
			H: faceSize,
		}

		cropSize := float64(max(face.W, face.H)) * faceBuffer
		cropCenterX := float64(face.X) + float64(face.W)*0.5
		cropCenterY := float64(face.Y) + float64(face.H)*0.5

		crop := render.Rect{
			X: cropCenterX - cropSize*0.5,
			Y: cropCenterY - cropSize*0.5,
			W: cropSize,
			H: cropSize,
		}

		scene.PhotoCrops = append(scene.PhotoCrops, crop)

		scene.Photos = append(scene.Photos, photo)
		allFaces = append(allFaces, face)

		x += faceSize + spacing
		if x+faceSize+spacing > layoutWidth {
			x = padding
			y += faceSize + spacing
		}
	}

	y += faceSize + spacing

	scene.Bounds = render.Rect{
		X: 0,
		Y: 0,
		W: layout.ViewportWidth,
		H: y + padding,
	}

	// Set custom region source that includes face bbox data
	scene.RegionSource = FaceRegionSource{
		Source: source,
		Faces:  allFaces,
	}
}
