//go:build embeddocs
// +build embeddocs

package main

import "embed"

//go:embed docs/.vitepress/dist
var StaticDocsFs embed.FS
var StaticDocsPath = "docs/.vitepress/dist"
