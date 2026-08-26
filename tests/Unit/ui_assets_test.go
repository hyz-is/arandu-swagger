package unit_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/hyz-is/arandu-swagger/internal/ui"
)

func TestSwaggerUIAssetsAreEmbeddedWithVersionedPathsAndMIMETypes(t *testing.T) {
	t.Parallel()

	expected := map[string]string{
		"assets/5.32.14/LICENSE":                          "text/plain; charset=utf-8",
		"assets/5.32.14/NOTICE":                           "text/plain; charset=utf-8",
		"assets/5.32.14/swagger-ui-bundle.js":             "text/javascript; charset=utf-8",
		"assets/5.32.14/swagger-ui-bundle.js.LICENSE.txt": "text/plain; charset=utf-8",
		"assets/5.32.14/swagger-ui.css":                   "text/css; charset=utf-8",
	}

	paths := ui.Paths()
	if !slices.IsSorted(paths) {
		t.Fatalf("asset paths are not sorted: %v", paths)
	}
	if len(paths) != len(expected) {
		t.Fatalf("got %d asset paths, want %d", len(paths), len(expected))
	}

	for _, path := range paths {
		contentType, ok := expected[path]
		if !ok {
			t.Errorf("unexpected asset path %q", path)
			continue
		}
		asset, ok := ui.Lookup(path)
		if !ok {
			t.Errorf("embedded asset %q was not found", path)
			continue
		}
		if asset.Path != path {
			t.Errorf("asset path = %q, want %q", asset.Path, path)
		}
		if asset.ContentType != contentType {
			t.Errorf("asset %q content type = %q, want %q", path, asset.ContentType, contentType)
		}
		if len(asset.Bytes) == 0 {
			t.Errorf("embedded asset %q is empty", path)
		}
	}
}

func TestSwaggerUIRuntimeAssetsAreSelfHosted(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"swagger-ui-bundle.js", "swagger-ui.css"} {
		path, ok := ui.Path(name)
		if !ok {
			t.Fatalf("runtime asset %q is not allowed", name)
		}
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "//") {
			t.Errorf("runtime asset %q uses a remote URL: %q", name, path)
		}

		asset, ok := ui.Lookup(path)
		if !ok {
			t.Fatalf("runtime asset %q was not found", path)
		}
		lower := bytes.ToLower(asset.Bytes)
		for _, host := range []string{"cdn.jsdelivr.net", "unpkg.com", "cdnjs.cloudflare.com"} {
			if bytes.Contains(lower, []byte(host)) {
				t.Errorf("runtime asset %q contains CDN host %q", path, host)
			}
		}
	}
}

func TestSwaggerUIAssetLookupRejectsUnknownAndTraversalPaths(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"",
		"swagger-ui.css",
		"/assets/5.32.14/swagger-ui.css",
		"assets/5.32.14/../5.32.14/swagger-ui.css",
		"assets/5.32.14/%2e%2e/swagger-ui.css",
		"assets/5.32.14/swagger-ui.css?download=1",
		"assets/5.32.15/swagger-ui.css",
		"assets/5.32.14/swagger-ui.css.map",
	}
	for _, path := range invalid {
		if _, ok := ui.Lookup(path); ok {
			t.Errorf("Lookup(%q) accepted a path outside the allowlist", path)
		}
	}

	for _, name := range []string{"../swagger-ui.css", "swagger-ui.css/extra", "swagger-ui.css?download=1"} {
		if _, ok := ui.Path(name); ok {
			t.Errorf("Path(%q) accepted an unknown asset name", name)
		}
	}
}

func TestSwaggerUILegalFilesMatchTheVendoredDistribution(t *testing.T) {
	t.Parallel()

	licensePath, _ := ui.Path("LICENSE")
	license, ok := ui.Lookup(licensePath)
	if !ok {
		t.Fatal("Swagger UI LICENSE was not embedded")
	}
	if !bytes.Contains(license.Bytes, []byte("Apache License")) || !bytes.Contains(license.Bytes, []byte("Version 2.0")) {
		t.Error("Swagger UI LICENSE does not contain the Apache-2.0 license text")
	}

	noticePath, _ := ui.Path("NOTICE")
	notice, ok := ui.Lookup(noticePath)
	if !ok {
		t.Fatal("Swagger UI NOTICE was not embedded")
	}
	if got, want := string(notice.Bytes), "swagger-ui\nCopyright 2020-2021 SmartBear Software Inc.\n"; got != want {
		t.Errorf("Swagger UI NOTICE = %q, want %q", got, want)
	}

	bundlePath, _ := ui.Path("swagger-ui-bundle.js")
	bundle, ok := ui.Lookup(bundlePath)
	if !ok {
		t.Fatal("Swagger UI bundle was not embedded")
	}
	if !bytes.HasPrefix(bundle.Bytes, []byte("/*! For license information please see swagger-ui-bundle.js.LICENSE.txt */")) {
		t.Error("Swagger UI bundle does not retain its license pointer")
	}
}
