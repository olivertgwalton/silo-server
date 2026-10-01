package imageutil

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/h2non/bimg"

	"github.com/Silo-Server/silo-server/internal/artworkkey"
)

func TestGenerateVariantsPreservesEncodes(t *testing.T) {
	for _, size := range [][2]int{{320, 180}, {500, 750}, {1920, 1080}, {500, 2400}, {2400, 1600}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			data := largeTestJPEG(t, size[0], size[1])
			widths := []int{300, 1920, 500, 780, 500}
			got, err := GenerateVariants(data, widths, artworkkey.DefaultOriginalMaxDimension)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(widths, []int{300, 1920, 500, 780, 500}) {
				t.Fatal("mutated requested widths")
			}
			if got.Ext != ".webp" || len(got.Variants) != 6 {
				t.Fatalf("unexpected result: %#v", got)
			}
			for i, width := range []int{0, 1920, 780, 500, 500, 300} {
				opts := bimg.Options{Type: bimg.WEBP, Quality: webpQuality, StripMetadata: true}
				key := "original"
				if i == 0 {
					fitWithin(&opts, bimg.ImageSize{Width: size[0], Height: size[1]}, artworkkey.DefaultOriginalMaxDimension)
				} else {
					key = fmt.Sprintf("w%d", width)
					if size[0] > width {
						opts.Width = width
					}
				}
				want, err := bimg.NewImage(data).Process(opts)
				if err != nil {
					t.Fatal(err)
				}
				if got.Variants[i].Key != key || !bytes.Equal(got.Variants[i].Data, want) {
					t.Fatalf("variant %d (%s) differs from independent encode", i, key)
				}
			}
			before := bytes.Clone(got.Variants[4].Data)
			got.Variants[3].Data[0] ^= 0xff
			if !bytes.Equal(got.Variants[4].Data, before) {
				t.Fatal("duplicate variants share mutable buffers")
			}
		})
	}
}

// TestGenerateVariantsCapsOriginalPerImageType keeps a 4K backdrop's original
// at 4K while every other type stays at 1920, and leaves the rungs untouched.
func TestGenerateVariantsCapsOriginalPerImageType(t *testing.T) {
	data := largeTestJPEG(t, 3840, 2160)
	wantRung, err := bimg.NewImage(data).Process(bimg.Options{Type: bimg.WEBP, Quality: webpQuality, StripMetadata: true, Width: 1920})
	if err != nil {
		t.Fatal(err)
	}
	for imageType, wantWidth := range map[string]int{
		artworkkey.ImageBackdrop: 3840,
		artworkkey.ImagePoster:   1920,
		artworkkey.ImageStill:    1920,
		artworkkey.ImageLogo:     1920,
		artworkkey.ImageProfile:  1920,
	} {
		t.Run(imageType, func(t *testing.T) {
			got, err := GenerateVariants(data, []int{1920}, artworkkey.OriginalMaxDimension(imageType))
			if err != nil {
				t.Fatal(err)
			}
			original, err := bimg.NewImage(got.Variants[0].Data).Size()
			if err != nil {
				t.Fatal(err)
			}
			if got.Variants[0].Key != "original" || original.Width != wantWidth || original.Height != wantWidth*9/16 {
				t.Fatalf("original %s = %dx%d, want %dx%d", got.Variants[0].Key, original.Width, original.Height, wantWidth, wantWidth*9/16)
			}
			if got.Variants[1].Key != "w1920" || !bytes.Equal(got.Variants[1].Data, wantRung) {
				t.Fatal("w1920 differs from an independent encode")
			}
		})
	}
}

func TestEncodeWebPWidthMatchesVariantRung(t *testing.T) {
	for _, size := range [][2]int{{1920, 1080}, {200, 120}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			data := largeTestJPEG(t, size[0], size[1])
			variants, err := GenerateVariants(data, []int{300}, artworkkey.DefaultOriginalMaxDimension)
			if err != nil {
				t.Fatal(err)
			}
			got, err := EncodeWebPWidth(data, 300)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, variants.Variants[1].Data) {
				t.Fatal("single-width encode differs from the w300 variant")
			}
			decoded, err := bimg.NewImage(got).Size()
			if err != nil {
				t.Fatal(err)
			}
			if want := min(size[0], 300); decoded.Width != want {
				t.Fatalf("width = %d, want %d", decoded.Width, want)
			}
		})
	}
}

func TestEncodeWebPWidthRejectsGarbage(t *testing.T) {
	if _, err := EncodeWebPWidth([]byte("not an image"), 300); err == nil {
		t.Fatal("expected an error for invalid image data")
	}
}
