package apiv2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	contracts "github.com/Silo-Server/silo-server/contracts/api/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/Silo-Server/silo-server/internal/api/handlers"
	"github.com/Silo-Server/silo-server/internal/config"
)

func TestOperationalDiscoveryPreservesCompatibilityView(t *testing.T) {
	cfg := &config.Config{}
	cfg.JellyfinCompat.Enabled = true
	cfg.JellyfinCompat.PublicURL = "https://compat.example.invalid"
	cfg.JellyfinCompat.ServerName = "Test server"
	service := handlers.NewCompatConnectInfoHandler(cfg, nil, nil)
	deps := parityDeps(false)
	deps.CompatConnectInfo = service
	handler := NewHandler(deps)
	response := do(t, handler, http.MethodGet, Prefix+"/compat/connect-info", "", bearer(memberToken))
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	assertDiscoverySchema(t, response, "CompatConnectInfoResponse")
	legacy := httptest.NewRecorder()
	service.HandleGetConnectInfo(legacy, httptest.NewRequest(http.MethodGet, "/api/v1/compat/connect-info", nil))
	var actual, want map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(legacy.Body.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("v2=%v legacy=%v", actual, want)
	}
	requireProblem(t, do(t, handler, http.MethodGet, Prefix+"/compat/connect-info", "", nil), TypeAuthenticationRequired)
	deps.CompatConnectInfo = nil
	requireProblem(t, do(t, NewHandler(deps), http.MethodGet, Prefix+"/compat/connect-info", "", bearer(memberToken)), TypeDependencyUnavailable)
}

func TestOperationalDiscoveryImageLadderAndOptionalProfile(t *testing.T) {
	handler := NewHandler(parityDeps(false))
	response := do(t, handler, http.MethodGet, Prefix+"/images/capabilities", "", bearer(memberToken))
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	assertDiscoverySchema(t, response, "ImageCapabilities")
	var actual ImageCapabilities
	if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	legacy := httptest.NewRecorder()
	handlers.HandleImagesCapability(legacy, httptest.NewRequest(http.MethodGet, "/api/v1/images/capability", nil))
	var want handlers.ImagesCapabilityResponse
	if err := json.Unmarshal(legacy.Body.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	if actual.SeasonListArtworkParam != want.SeasonListArtworkParam || actual.SeasonListArtworkParam != "include_artwork" || actual.State != StateAvailable || actual.Revision == "" || actual.Param != want.Param || actual.OriginalMaxWidthPx != want.OriginalMaxWidthPx || !reflect.DeepEqual(actual.Sizes, want.Sizes) || len(actual.Widths) != len(want.Widths) {
		t.Fatalf("v2=%+v legacy=%+v", actual, want)
	}
	for key, expected := range want.Widths {
		width, ok := actual.Widths[key]
		if !ok || width.Small != expected.Small || width.Medium != expected.Medium || width.Large != expected.Large {
			t.Fatalf("image width %s: got %+v, want %+v", key, width, expected)
		}
		// The scalar stays a true upper bound over every per-type original.
		if width.OriginalMaxPx < width.Large || width.OriginalMaxPx > actual.OriginalMaxWidthPx {
			t.Fatalf("image width %s: original_max_px %d outside [large %d, original_max_width_px %d]", key, width.OriginalMaxPx, width.Large, actual.OriginalMaxWidthPx)
		}
	}
	if actual.OriginalMaxWidthPx != 3840 || actual.Widths["backdrop"].OriginalMaxPx != 3840 || actual.Widths["poster"].OriginalMaxPx != 1920 {
		t.Fatalf("original bounds: scalar %d, backdrop %d, poster %d; want 3840, 3840, 1920", actual.OriginalMaxWidthPx, actual.Widths["backdrop"].OriginalMaxPx, actual.Widths["poster"].OriginalMaxPx)
	}
	requireProblem(t, do(t, handler, http.MethodGet, Prefix+"/images/capabilities", "", nil), TypeAuthenticationRequired)
	requireProblem(t, do(t, handler, http.MethodGet, Prefix+"/images/capabilities", "", with(bearer(memberToken), "X-Profile-Id", "p-locked")), TypeProfileVerificationRequired)
	requireProblem(t, do(t, handler, http.MethodGet, Prefix+"/images/capabilities?unknown=value", "", bearer(memberToken)), TypeValidationFailed)
}

func assertDiscoverySchema(t *testing.T, response *httptest.ResponseRecorder, name string) {
	t.Helper()
	var document, body any
	if err := json.Unmarshal(contracts.OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	const url = "https://schema.example.invalid/openapi.json"
	if err := compiler.AddResource(url, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(url + "#/components/schemas/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(body); err != nil {
		t.Fatal(err)
	}
}
