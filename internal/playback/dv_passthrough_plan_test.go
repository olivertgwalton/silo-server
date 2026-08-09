package playback

import "testing"

func dvPassthroughRegistry(available bool) *TransformationRegistryV3 {
	return NewTransformationRegistryV3([]TransformationSpecV3{
		{Name: "server_dv_passthrough", RecipeVersion: "1", Available: available},
	})
}

func dvPassthroughRequest(profiles ...int) StartRequestV3 {
	return StartRequestV3{
		Capabilities: ClientCodecCapabilitiesV3{
			HDRDetails: &HDRCapabilitiesV3{DolbyVisionProfiles: profiles},
		},
	}
}

// Single-layer profiles survive a base-layer-only copy intact, so they are the
// only ones eligible for passthrough.
func TestCanPreserveDolbyVisionAcceptsSingleLayerProfiles(t *testing.T) {
	for _, profile := range []int{5, 8} {
		source := SourceDescriptorV3{DynamicRange: "dolby_vision", DVProfile: profile}
		if !canPreserveDolbyVisionV3(source, dvPassthroughRequest(5, 8), dvPassthroughRegistry(true)) {
			t.Fatalf("profile %d should be preservable", profile)
		}
	}
}

// Profile 7's enhancement layer is dropped by the remux's stream mapping, so
// preserving it would leave dangling RPUs. It belongs on the strip or the
// client conversion instead.
func TestCanPreserveDolbyVisionRejectsProfile7(t *testing.T) {
	source := SourceDescriptorV3{DynamicRange: "dolby_vision", DVProfile: 7}
	if canPreserveDolbyVisionV3(source, dvPassthroughRequest(5, 7, 8), dvPassthroughRegistry(true)) {
		t.Fatal("profile 7 must not take the passthrough route")
	}
}

// Below FFmpeg 8 the copy drops the configuration record, so the route must
// stay off the table rather than promise Dolby Vision it cannot deliver.
func TestCanPreserveDolbyVisionRequiresCapableToolchain(t *testing.T) {
	source := SourceDescriptorV3{DynamicRange: "dolby_vision", DVProfile: 5}
	if canPreserveDolbyVisionV3(source, dvPassthroughRequest(5), dvPassthroughRegistry(false)) {
		t.Fatal("passthrough must require the dv copy tagging capability")
	}
	if canPreserveDolbyVisionV3(source, dvPassthroughRequest(5), nil) {
		t.Fatal("passthrough must require a registry")
	}
}

// A client that never declared the profile must not be handed it.
func TestCanPreserveDolbyVisionRequiresClientProfileSupport(t *testing.T) {
	source := SourceDescriptorV3{DynamicRange: "dolby_vision", DVProfile: 5}
	if canPreserveDolbyVisionV3(source, dvPassthroughRequest(8), dvPassthroughRegistry(true)) {
		t.Fatal("profile 5 source offered to a client declaring only profile 8")
	}
}

// Non-DV sources must never acquire a Dolby Vision claim.
func TestCanPreserveDolbyVisionIgnoresNonDolbyVisionSources(t *testing.T) {
	source := SourceDescriptorV3{DynamicRange: "hdr10", DVProfile: 0}
	if canPreserveDolbyVisionV3(source, dvPassthroughRequest(5, 8), dvPassthroughRegistry(true)) {
		t.Fatal("hdr10 source claimed Dolby Vision")
	}
}
