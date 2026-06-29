package main

import (
	"fmt"
	"time"
)

type ViewerAsset struct {
	Path      string
	MediaType string
}

func main() {
	asset := ViewerAsset{
		Path:      "examples/browser-preview.pdf",
		MediaType: "application/pdf",
	}

	fmt.Printf("Preview %s as %s at %s\n", asset.Path, asset.MediaType, time.Now().Format(time.RFC3339))
}
