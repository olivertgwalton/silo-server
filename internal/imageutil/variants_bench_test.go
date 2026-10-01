package imageutil

import (
	"fmt"
	"testing"

	"github.com/Silo-Server/silo-server/internal/artworkkey"
)

func BenchmarkGenerateVariants(b *testing.B) {
	for _, size := range [][2]int{{320, 180}, {500, 750}, {1920, 1080}} {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			data := largeTestJPEG(b, size[0], size[1])
			imageType := artworkkey.ImagePoster
			if size[0] == 1920 {
				imageType = artworkkey.ImageBackdrop
			}
			widths := artworkkey.VariantWidths(imageType)
			originalMaxDim := artworkkey.OriginalMaxDimension(imageType)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := GenerateVariants(data, widths, originalMaxDim); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
