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
		Enabled: true,
		Title:   "API Peráta",
		Version: "1.0.0",
		Locale:  "pt-BR",
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
	if !strings.Contains(initBody, `"Try it out":"Testar"`) {
		t.Errorf("expected default pt-BR \"Try it out\":\"Testar\" in initializer, got: %s", initBody)
	}
	if !strings.Contains(initBody, `"Parameters":"Parâmetros"`) {
		t.Errorf("expected default pt-BR \"Parameters\":\"Parâmetros\" in initializer, got: %s", initBody)
	}
	if !strings.Contains(initBody, `translateNode(container)`) {
		t.Errorf("expected translateNode invocation in initializer, got: %s", initBody)
	}
}
