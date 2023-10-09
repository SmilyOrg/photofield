//go:build embedstatic
// +build embedstatic

package main

import "embed"

//go:embed ui/dist
var StaticFs embed.FS
