//go:build embeddocs
// +build embeddocs

package main

import "embed"

//go:embed ui/docs/.vitepress/dist
var StaticDocsFs embed.FS
var StaticDocsPath = "ui/docs/.vitepress/dist"
