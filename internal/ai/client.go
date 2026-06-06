package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
	"time"
	"unsafe"
)

var ErrNotAvailable = errors.New("AI server host not configured")

type encodedEmbedding struct {
	EmbeddingF16B64     string `json:"embedding_f16_b64,omitempty"`
	EmbeddingInvNormU16 uint16 `json:"embedding_inv_norm_f16_uint16,omitempty"`
}

type Model struct {
	Host string `json:"host"`
}

type AI struct {
	Host    string `json:"host"`
	Visual  Model  `json:"visual"`
	Textual Model  `json:"textual"`
	Faces   Model  `json:"faces"`

	facesAvailable bool
	facesChecked   bool
	facesMu        sync.RWMutex
}

func (a AI) Available() bool {
	return a.TextualHost() != ""
}

func (a *AI) CheckFacesAvailable() {
	if !a.Available() || a.Host == "" {
		return
	}

	url := fmt.Sprintf("%s/faces", a.Host)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		a.facesMu.Lock()
		a.facesAvailable = false
		a.facesChecked = true
		a.facesMu.Unlock()
		return
	}
	defer res.Body.Close()

	a.facesMu.Lock()
	a.facesAvailable = res.StatusCode == http.StatusOK
	a.facesChecked = true
	a.facesMu.Unlock()
}

func (a *AI) FacesAvailable() bool {
	a.facesMu.RLock()
	checked := a.facesChecked
	available := a.facesAvailable
	a.facesMu.RUnlock()

	if !checked {
		return false
	}
	return available
}

func (a AI) VisualHost() string {
	if a.Visual.Host != "" {
		return a.Visual.Host
	}
	return a.Host
}

func (a AI) TextualHost() string {
	if a.Textual.Host != "" {
		return a.Textual.Host
	}
	return a.Host
}

func (a AI) FaceHost() string {
	if a.Faces.Host != "" {
		return a.Faces.Host
	}
	return a.Host
}

func (a AI) EmbedImagePath(path string) (Embedding, error) {
	if !a.Available() || a.TextualHost() == "" {
		return nil, ErrNotAvailable
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 50000000 {
		return nil, errors.New("file too big")
	}
	return a.EmbedImageReader(f)
}

func (a AI) EmbedImageReader(r io.Reader) (Embedding, error) {
	if !a.Available() || a.VisualHost() == "" {
		return nil, ErrNotAvailable
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("image", "image")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(fw, r)
	if err != nil {
		return nil, err
	}

	w.Close()

	url := fmt.Sprintf("%s/image-embeddings", a.VisualHost())
	res, err := http.Post(url, w.FormDataContentType(), &b)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	var response struct {
		Images []struct {
			Field    string `json:"field"`
			Filename string `json:"filename"`
			encodedEmbedding
		} `json:"images"`
	}
	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Images) == 0 {
		return nil, errors.New("missing images")
	}

	bytes, err := base64.StdEncoding.DecodeString(response.Images[0].EmbeddingF16B64)
	if err != nil {
		return nil, err
	}
	invnorm := response.Images[0].EmbeddingInvNormU16

	return embedding{
		bytes:   bytes,
		invnorm: invnorm,
	}, nil
}

func (a AI) EmbedText(text string) (Embedding, error) {
	if !a.Available() {
		return nil, ErrNotAvailable
	}

	b, err := json.Marshal(
		struct {
			Texts []string `json:"texts"`
		}{
			Texts: []string{text},
		},
	)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/text-embeddings", a.TextualHost())
	res, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	var response struct {
		Texts []struct {
			Text string `json:"text"`
			encodedEmbedding
		} `json:"texts"`
	}
	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Texts) == 0 {
		return nil, errors.New("missing data")
	}

	bytes, err := base64.StdEncoding.DecodeString(response.Texts[0].EmbeddingF16B64)
	if err != nil {
		return nil, err
	}
	invnorm := response.Texts[0].EmbeddingInvNormU16

	return embedding{
		bytes:   bytes,
		invnorm: invnorm,
	}, nil
}

// Face represents a detected face with bounding box and embedding
type Face struct {
	X          int    // X coordinate (pixels)
	Y          int    // Y coordinate (pixels)
	W          int    // Width (pixels)
	H          int    // Height (pixels)
	Confidence int    // Confidence 0-100
	Embedding  []byte // Normalized face embedding
}

func (a AI) DetectFaces(r io.Reader) ([]Face, error) {
	if !a.Available() || a.FaceHost() == "" || !a.FacesAvailable() {
		return nil, ErrNotAvailable
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("image", "image")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(fw, r)
	if err != nil {
		return nil, err
	}

	w.Close()

	url := fmt.Sprintf("%s/faces", a.FaceHost())
	res, err := http.Post(url, w.FormDataContentType(), &b)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)

	var response struct {
		Images []struct {
			Faces []struct {
				BBox       [4]float64 `json:"bbox"`
				Confidence float64    `json:"confidence"`
				encodedEmbedding
			} `json:"faces"`
		} `json:"images"`
	}
	err = decoder.Decode(&response)
	if err != nil {
		return nil, err
	}

	if len(response.Images) == 0 {
		return nil, errors.New("missing images")
	}

	apifaces := response.Images[0].Faces
	faces := make([]Face, 0, len(apifaces))
	for _, f := range apifaces {
		embBytes, err := base64.StdEncoding.DecodeString(f.EmbeddingF16B64)
		if err != nil {
			return nil, err
		}

		// Convert confidence from 0-1 to 0-100
		confidence := min(100, max(0, int(f.Confidence*100)))

		faces = append(faces, Face{
			X:          int(f.BBox[0]),
			Y:          int(f.BBox[1]),
			W:          int(f.BBox[2] - f.BBox[0]),
			H:          int(f.BBox[3] - f.BBox[1]),
			Confidence: confidence,
			Embedding:  embBytes,
		})
	}

	return faces, nil
}

func Float16SliceToFloat(bytes []byte) []Float {
	if len(bytes) == 0 {
		return nil
	}
	p := unsafe.Pointer(&bytes[0])
	return unsafe.Slice((*Float)(p), len(bytes)/2)
}
