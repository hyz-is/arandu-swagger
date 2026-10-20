package feature_test

import (
	"encoding/json"
	"net/http"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	swagger "github.com/hyz-is/arandu-swagger"
)

func TestServedSpecificationCarriesTheRegisteredWebhooks(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.DisableUI = true
	config.CacheSpec = true
	config.Filter = swagger.RouteFilter{IncludePrefixes: []string{"/api/"}}
	router, module := mount(t, config)

	orders := router.ForModule("orders").Get("/api/orders", emptyHandler).Name("orders.index")
	module.Route(orders).Response(http.StatusOK, "Orders.")
	if err := module.Schema("OrderEvent", jsonschema.Object(jsonschema.Prop("id", jsonschema.String().Required()))); err != nil {
		t.Fatal(err)
	}
	webhook := func(name string) swagger.Webhook {
		return swagger.Webhook{
			Name:      name,
			Tags:      []string{"webhooks"},
			Body:      []*swagger.Media{swagger.JSONRef("OrderEvent")},
			Responses: swagger.Responses{"2XX": {Description: "Received."}},
		}
	}
	if err := module.Webhooks(webhook("order.paid")); err != nil {
		t.Fatal(err)
	}

	first := servedWebhooks(t, router)
	if _, ok := first["order.paid"]["post"]; !ok {
		t.Fatalf("served webhooks = %v, want order.paid as a POST despite the route filter", first)
	}

	// A webhook registered after the first answer reaches the next one: the
	// cached document follows the registry revision.
	if err := module.Webhooks(webhook("order.refunded")); err != nil {
		t.Fatal(err)
	}
	second := servedWebhooks(t, router)
	if _, ok := second["order.refunded"]; !ok || len(second) != 2 {
		t.Fatalf("served webhooks after a late registration = %v", second)
	}
}

func servedWebhooks(t *testing.T, router *fhttp.Router) map[string]map[string]any {
	t.Helper()
	response := request(router, swagger.DefaultSpecPath)
	if response.Code != http.StatusOK {
		t.Fatalf("specification answered %d: %s", response.Code, response.Body.String())
	}
	var document struct {
		OpenAPI  string                               `json:"openapi"`
		Webhooks map[string]map[string]any            `json:"webhooks"`
		Paths    map[string]map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode specification: %v", err)
	}
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("openapi = %q, webhooks need 3.1", document.OpenAPI)
	}
	if _, ok := document.Paths["/api/orders"]; !ok {
		t.Fatalf("paths = %v, the route table is no longer the source of the paths", document.Paths)
	}
	return document.Webhooks
}
