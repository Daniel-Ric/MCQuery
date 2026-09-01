package cli

import (
	"strings"
	"testing"
)

func TestFormatUpdateInfoHandlesRepositoryWithoutReleases(t *testing.T) {
	text := formatUpdateInfo(updateInfo{
		CurrentVersion:   "0.2.0",
		LatestURL:        updateRepoURL,
		Source:           "repository",
		VersionPublished: false,
	})

	for _, expected := range []string{
		"Current version: 0.2.0",
		"Published version: none",
		"no release or version tag has been published yet",
		updateRepoURL,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("update info is missing %q in %q", expected, text)
		}
	}
	if strings.Contains(text, "no tags found") {
		t.Fatalf("raw GitHub fallback error leaked into the UI: %q", text)
	}
}

func TestFormatUpdateInfoShowsPublishedUpdate(t *testing.T) {
	text := formatUpdateInfo(updateInfo{
		CurrentVersion:   "0.2.0",
		LatestVersion:    "0.3.0",
		LatestURL:        "https://example.test/release",
		Source:           "release",
		UpdateAvailable:  true,
		VersionPublished: true,
	})

	if !strings.Contains(text, "Latest version: 0.3.0") || !strings.Contains(text, "Status: update available") {
		t.Fatalf("unexpected published update text: %q", text)
	}
}
