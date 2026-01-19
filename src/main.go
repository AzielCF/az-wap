package main

import (
	"embed"

	"github.com/AzielCF/az-wap/cmd"
)

//go:embed frontend/dist
var embedFrontend embed.FS

func main() {
	cmd.Execute(embedFrontend)
}
