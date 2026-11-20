package feature_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestSwaggerUII18nAndCustomTranslations(t *testing.T) {
	cfg := swagger.Config{
		Enabled:  true,
		ViewName: unpublishedView,
		Title:    "API Peráta",
		Version:  "1.0.0",
		Locale:   "pt-BR",
		Translations: map[string]string{
			"Authorize": "Fazer Login",
		},
	}

	module, err := swagger.New(cfg)
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	router := fhttp.NewRouter()
	module.Routes(router)

	// Test HTML shell has lang="pt-BR"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /docs, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="pt-BR">`) {
		t.Errorf("expected <html lang=\"pt-BR\"> in response, got: %s", body)
	}

	// Test swagger-initializer.js contains translation dictionary and overrides
	initRec := httptest.NewRecorder()
	initReq := httptest.NewRequest(http.MethodGet, "/docs/swagger-initializer.js", nil)
	router.ServeHTTP(initRec, initReq)

	if initRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for swagger-initializer.js, got %d", initRec.Code)
	}
	initBody := initRec.Body.String()
	if !strings.Contains(initBody, `"Authorize":"Fazer Login"`) {
		t.Errorf("expected custom override \"Authorize\":\"Fazer Login\" in initializer, got: %s", initBody)
	}
	for _, want := range []string{`"Expand all":"Expandir tudo"`, `"Collapse all":"Recolher tudo"`} {
		if !strings.Contains(initBody, want) {
			t.Errorf("expected default pt-BR %s in initializer, got: %s", want, initBody)
		}
	}
	if !strings.Contains(initBody, `"Try it out":"Testar"`) {
		t.Errorf("expected default pt-BR \"Try it out\":\"Testar\" in initializer, got: %s", initBody)
	}
	if !strings.Contains(initBody, `"Parameters":"Parâmetros"`) {
		t.Errorf("expected default pt-BR \"Parameters\":\"Parâmetros\" in initializer, got: %s", initBody)
	}
	if !strings.Contains(initBody, `translateNode(container)`) {
		t.Errorf("expected translateNode invocation in initializer, got: %s", initBody)
	}
	for _, want := range []string{`"Edit Value":"Editar valor"`, `"JSON Schema dialect:":"Dialeto do JSON Schema:"`} {
		if !strings.Contains(initBody, want) {
			t.Errorf("expected default pt-BR %s in initializer, got: %s", want, initBody)
		}
	}
}

// React writes a label that changed (Try it out, then Cancel) into the text node
// it already has, which adds no node, so an observer that only watches children
// added never translates it. The initializer watches the text of existing nodes
// too, and never writes a text that is already the translation, which would
// queue a record for itself forever.
func TestSwaggerUITranslatesLabelsReactRewritesInPlace(t *testing.T) {
	module, err := swagger.New(swagger.Config{Enabled: true, ViewName: unpublishedView, Title: "API", Version: "1.0.0", Locale: "pt-BR"})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}
	router := fhttp.NewRouter()
	module.Routes(router)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/swagger-initializer.js", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`m.type === 'characterData'`,
		`translateText(m.target)`,
		`observer.observe(container, { childList: true, subtree: true, characterData: true })`,
		`observer.observe(document.body, { childList: true, subtree: true, characterData: true })`,
		`if (next !== current) node.nodeValue = next;`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the initializer lacks %q:\n%s", want, body)
		}
	}
}
