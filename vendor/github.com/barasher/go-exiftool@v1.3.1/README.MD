# go-exiftool

[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![Build Status](https://travis-ci.org/barasher/go-exiftool.svg?branch=master)](https://travis-ci.org/barasher/go-exiftool)
[![go report card](https://goreportcard.com/badge/github.com/barasher/go-exiftool "go report card")](https://goreportcard.com/report/github.com/barasher/go-exiftool)
[![GoDoc](https://godoc.org/github.com/barasher/go-exiftool?status.svg)](https://godoc.org/github.com/barasher/go-exiftool)
[![codecov](https://codecov.io/gh/barasher/go-exiftool/branch/master/graph/badge.svg)](https://codecov.io/gh/barasher/go-exiftool)

**go-exiftool** is a golang library that wraps [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/).

[ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s purpose is to extract as much metadata as possible (EXIF, IPTC, XMP, GPS, ...) from [a lots of differents file types](https://www.sno.phy.queensu.ca/~phil/exiftool/#supported) (Office documents, pictures, movies, PDF, ...).

**go-exiftool** uses [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s *`stay_open`* feature to [optimize performance](https://www.sno.phy.queensu.ca/~phil/exiftool/#performance).

## Requirements

**go-exiftool** needs [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/) to be installed.

- On Debian : `sudo apt-get install exiftool`

## Usage

```go
et, err := exiftool.NewExiftool()
if err != nil {
    fmt.Printf("Error when intializing: %v\n", err)
    return
}
defer et.Close()

fileInfos := et.ExtractMetadata("testdata/20190404_131804.jpg")

for _, fileInfo := range fileInfos {
    if fileInfo.Err != nil {
        fmt.Printf("Error concerning %v: %v\n", fileInfo.File, fileInfo.Err)
        continue
    }

    for k, v := range fileInfo.Fields {
        fmt.Printf("[%v] %v\n", k, v)
    }
}
```

Output :

```
[FOV] 69.4 deg
[Orientation] Rotate 90 CW
[ColorSpace] sRGB
[Compression] JPEG (old-style)
[YCbCrSubSampling] YCbCr4:2:2 (2 1)
[Aperture] 1.7
[ColorComponents] 3
[SubSecCreateDate] 2019:04:04 13:18:03.0937
[FileSize] 26 kB
[FileAccessDate] 2019:05:17 22:44:26+02:00
[DateTimeOriginal] 2019:04:04 13:18:03
[CreateDate] 2019:04:04 13:18:03
(...)
```

## Changelog

- v1.1.0 : initial release
- [v1.1.1](https://github.com/barasher/go-exiftool/milestone/2)
  - [add typed getters for filemetadata fields](https://github.com/barasher/go-exiftool/issues/2)
- [v1.1.3](https://github.com/barasher/go-exiftool/milestone/3)
  - [add Mac & Windows support](https://github.com/barasher/go-exiftool/pull/7) (thank to @PROger4ever)
  - [increase errors readability](https://github.com/barasher/go-exiftool/pull/8) (thank to @PROger4ever)
- [v1.2.0](https://github.com/barasher/go-exiftool/milestone/4)
  - [add stdout and stderr buffer configuration capabilities](https://github.com/barasher/go-exiftool/issues/6) (thank to @asannikov)
- [v1.3.0](https://github.com/barasher/go-exiftool/milestone/5)
  - Add functionnal option to set [ExifTool](https://www.sno.phy.queensu.ca/~phil/exiftool/)'s `-charset` parameter (thank to @PROger4ever)
- [v1.3.1](https://github.com/barasher/go-exiftool/milestone/6)
  - [fix line separator on macOS](https://github.com/barasher/go-exiftool/issues/16)