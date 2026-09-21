package feature_test

import (
	"slices"
	"testing"

	"github.com/arandu-io/framework/foundation"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestModuleImplementsPublishable(t *testing.T) {
	cfg := swagger.Config{
		Enabled: true,
		Title:   "Test API",
		Version: "1.0.0",
	}

	module, err := swagger.New(cfg)
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	publishable, ok := any(module).(foundation.Publishable)
	if !ok {
		t.Fatal("Module must implement foundation.Publishable")
	}

	publications := publishable.Publishes()
	if len(publications) == 0 {
		t.Fatal("expected at least one publication")
	}

	foundView := false
	for _, pub := range publications {
		if pub.Tag == foundation.PublishView {
			foundView = true
		}
	}
	if !foundView {
		t.Errorf("expected publication with tag %q", foundation.PublishView)
	}

	paths := swagger.PublishedPaths()
	expectedPaths := []string{
		"resources/views/docs/swagger.kyse.go",
		"resources/views/docs/topbar.kyse.go",
		"resources/views/docs/container.kyse.go",
		"resources/views/docs/header.kyse.go",
	}
	for _, expected := range expectedPaths {
		if !slices.Contains(paths, expected) {
			t.Errorf("expected PublishedPaths to contain %q, got: %v", expected, paths)
		}
	}
}
