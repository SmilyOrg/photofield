//go:build embedgeo
// +build embedgeo

package main

import "embed"

// tinygpkg-data release: v0.2.0
//go:embed data/geo/geoBoundariesCGAZ_ADM2_s5_twkb_p3.gpkg
var GeoFs embed.FS
