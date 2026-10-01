package jellycompat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Silo-Server/silo-server/internal/catalog"
)

func TestCompatRequestImageSize(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		imageType string
		want      string
	}{
		{"no request defaults to card", "", "Primary", "small"},
		{"no request backdrop defaults to medium", "", "Backdrop", "medium"},
		{"no dimensions", "/Items/1/Images/Primary", "Primary", "small"},
		{"no dimensions backdrop", "/Items/1/Images/Backdrop", "Backdrop", "medium"},
		{"thumbnail width", "/Items/1/Images/Primary?MaxWidth=200", "Primary", "small"},
		{"card boundary", "/Items/1/Images/Primary?FillWidth=320", "Primary", "small"},
		{"between card and large", "/Items/1/Images/Primary?MaxWidth=600", "Primary", "medium"},
		{"just below large", "/Items/1/Images/Primary?MaxWidth=779", "Primary", "medium"},
		{"large boundary", "/Items/1/Images/Primary?MaxWidth=780", "Primary", "large"},
		// The large bucket is a width rung, so a height-only constraint must not
		// reach it: a portrait poster at MaxHeight=900 would come back far taller
		// than the client asked for.
		{"height only does not reach large", "/Items/1/Images/Primary?MaxHeight=900", "Primary", "medium"},
		{"width reaches large", "/Items/1/Images/Primary?MaxWidth=900", "Primary", "large"},
		{"fill width reaches large", "/Items/1/Images/Primary?FillWidth=900", "Primary", "large"},
		{"fill height only does not reach large", "/Items/1/Images/Primary?FillHeight=900", "Primary", "medium"},
		{"fill height only backdrop stays medium", "/Items/1/Images/Backdrop?FillHeight=1100", "Backdrop", "medium"},
		{"large from fill width", "/Items/1/Images/Backdrop?FillWidth=1100", "Backdrop", "large"},
		// The pre-existing original bucket keeps reading any dimension; it serves
		// the stored original rather than a width rung.
		{"height only still reaches original", "/Items/1/Images/Primary?MaxHeight=1200", "Primary", "original"},
		{"just below original", "/Items/1/Images/Backdrop?MaxWidth=1199", "Backdrop", "large"},
		{"original boundary", "/Items/1/Images/Primary?MaxWidth=1200", "Primary", "original"},
		// A backdrop's original is up to 4K, so it is served only to a request
		// wider than the widest backdrop rung; up to that rung the request gets
		// the w1920 bytes it got when the original was capped at 1920.
		{"backdrop at the widest rung", "/Items/1/Images/Backdrop?maxWidth=1920", "Backdrop", "large"},
		{"thumb at the widest rung", "/Items/1/Images/Thumb?MaxWidth=1920", "Thumb", "large"},
		{"backdrop height at the widest rung", "/Items/1/Images/Backdrop?MaxHeight=1500", "Backdrop", "large"},
		{"backdrop beyond the widest rung", "/Items/1/Images/Backdrop?MaxWidth=1921", "Backdrop", "original"},
		{"backdrop at 4K", "/Items/1/Images/Backdrop?maxWidth=3840", "Backdrop", "original"},
		{"primary at 1920 keeps original", "/Items/1/Images/Primary?MaxWidth=1920", "Primary", "original"},
		{"logo at 1920 keeps original", "/Items/1/Images/Logo?MaxWidth=1920", "Logo", "original"},
		{"very large", "/Items/1/Images/Backdrop?MaxWidth=4000", "Backdrop", "original"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.query != "" {
				req = httptest.NewRequest(http.MethodGet, tt.query, nil)
			}
			if got := compatRequestImageSize(req, tt.imageType); got != tt.want {
				t.Fatalf("compatRequestImageSize(%q, %q) = %q, want %q", tt.query, tt.imageType, got, tt.want)
			}
		})
	}
}

// TestImageURLForItemKeepsPrimaryBackdropFallbackOffTheOriginal: a Primary
// request falls back to the backdrop for an item with no poster, and keeps the
// widest rung even when it asked for the original, so a client asking for a
// poster never receives a 4K backdrop. A Backdrop request may.
func TestImageURLForItemKeepsPrimaryBackdropFallbackOffTheOriginal(t *testing.T) {
	const backdrop = "tmdb/movies/550/backdrop/original.abc123.webp"
	for _, tc := range []struct {
		imageType string
		want      string
	}{
		{"Primary", "tmdb/movies/550/backdrop/w1920.abc123.webp"},
		{"Backdrop", backdrop},
	} {
		t.Run(tc.imageType, func(t *testing.T) {
			resolver := &recordingImageResolver{}
			detailSvc := &catalog.DetailService{}
			detailSvc.SetImageResolver(resolver)
			h := &ImagesHandler{detailSvc: detailSvc}

			h.imageURLForItem(context.Background(), "", "poster", backdrop, "", tc.imageType, compatOriginalImageSize)

			if resolver.path != tc.want {
				t.Fatalf("resolved path = %q, want %q", resolver.path, tc.want)
			}
		})
	}
}
