package imagemagick

import (
	"bytes"
	"fmt"
	"strconv"
)

func readInt(buf *bytes.Buffer, delim byte) (int, error) {
	s, err := buf.ReadString(delim)
	if err != nil {
		return 0, fmt.Errorf("unable to read: %w", err)
	}
	return strconv.Atoi(s[:len(s)-1])
}

var pam_prefix_magic = []byte("P7\n")
var pam_header_end = []byte("ENDHDR\n")

type pamImage struct {
	Width     int
	Height    int
	Depth     int
	MaxValue  int
	TupleType string
	Bytes     []byte
}

func readPAM(b []byte) (pamImage, error) {

	var img pamImage

	if !bytes.HasPrefix(b, pam_prefix_magic) {
		return img, fmt.Errorf("expected magic prefix %v", pam_prefix_magic)
	}

	b = b[len(pam_prefix_magic):]
	header := b[:256]
	buf := bytes.NewBuffer(header)

	for {
		key, err := buf.ReadString(' ')
		if err != nil {
			return img, err
		}
		key = key[:len(key)-1]
		switch key {
		case "WIDTH":
			img.Width, err = readInt(buf, '\n')
		case "HEIGHT":
			img.Height, err = readInt(buf, '\n')
		case "DEPTH":
			img.Depth, err = readInt(buf, '\n')
		case "MAXVAL":
			img.MaxValue, err = readInt(buf, '\n')
		case "TUPLTYPE":
			img.TupleType, err = buf.ReadString('\n')
		default:
			return img, fmt.Errorf("unexpected key: %s", key)
		}
		if err != nil {
			return img, err
		}
		if key == "TUPLTYPE" {
			end := buf.Bytes()
			if !bytes.HasPrefix(end, pam_header_end) {
				return img, fmt.Errorf("expected end of header marker")
			}
			start := len(header) - buf.Len() + len(pam_header_end)
			// println("len", buf.Len(), start)
			// fmt.Printf(">%s<\n", b[start:100])
			img.Bytes = b[start:]
			break
		}
	}
	return img, nil
}
