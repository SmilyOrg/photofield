package clip

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
	"unsafe"
)

var ErrNotAvailable = errors.New("AI server host not configured")

type encodedEmbedding struct {
	EmbeddingF16B64     string `json:"embedding_f16_b64,omitempty"`
	EmbeddingInvNormU16 uint16 `json:"embedding_inv_norm_f16_uint16,omitempty"`
}

type embedding struct {
	bytes   []byte
	invnorm uint16
}

func (e embedding) Byte() []byte {
	return e.bytes
}

func (e embedding) Float() []Float {
	if e.bytes == nil || len(e.bytes) == 0 {
		return nil
	}
	p := unsafe.Pointer(&e.bytes[0])
	return unsafe.Slice((*Float)(p), len(e.bytes)/2)
}

func (e embedding) Float32() []float32 {
	floats := e.Float()
	if floats == nil {
		return nil
	}
	l := len(floats)
	f32 := make([]float32, l)
	for i := 0; i < l; i++ {
		f32[i] = floats[i].Float32()
	}
	return f32
}

func (e embedding) InvNormUint16() uint16 {
	return e.invnorm
}

func (e embedding) InvNormFloat32() float32 {
	return Float(e.invnorm).Float32()
}

func FromRaw(bytes []byte, invnorm uint16) Embedding {
	return embedding{
		bytes:   bytes,
		invnorm: invnorm,
	}
}

type Model struct {
	Host string `json:"host"`
}

type AI struct {
	Host    string `json:"host"`
	Visual  Model  `json:"visual"`
	Textual Model  `json:"textual"`
}

func (a AI) Available() bool {
	return a.TextualHost() != ""
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
	if info.Size() > 20000000 {
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
