//go:build embedui
// +build embedui

package main

import "embed"

//go:embed ui/dist
var StaticFs embed.FS
