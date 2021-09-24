//go:build !embedstatic
// +build !embedstatic

package main

import "embed"

var StaticFs embed.FS
