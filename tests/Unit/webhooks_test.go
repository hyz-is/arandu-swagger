package unit_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	swagger "github.com/hyz-is/arandu-swagger"
)

type webhookOrder struct {
	ID          string `json:"id"`
	AmountCents int64  `json:"amount_cents"`
}

type webhookEvent struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	Data      webhookOrder `json:"data"`
}

// webhookCatalogue stands for an application's list of the events it sends.
var webhookCatalogue = []string{"order.paid", "order.created", "order.refunded"}

func webhooksFromCatalogue(deprecated map[string]bool) []swagger.Webhook {
	out := make([]swagger.Webhook, 0, len(webhookCatalogue))
	for _, name := range webhookCatalogue {
		out = append(out, swagger.Webhook{
			Name:        name,
			OperationID: "webhooks:" + name,
			Summary:     "Sent when " + name + " happens.",
			Tags:        []string{"webhooks"},
			Body:        []*swagger.Media{swagger.JSONRef("OrderEvent")},
			Responses:   swagger.Responses{"2XX": {Description: "Received."}},
			Deprecated:  deprecated[name],
		})
	}
	return out
}

func TestWebhooksFromATypedCatalogueAreTheDocumentsWebhooksSection(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/api/orders", func(http.ResponseWriter, *http.Request) {}).Name("orders.index")
	registry := swagger.NewRegistry()
	if err := registry.Models(swagger.ModelOf[webhookOrder]("Order"), swagger.ModelOf[webhookEvent]("OrderEvent")); err != nil {
		t.Fatal(err)
	}
	registry.Route(route).Response(http.StatusOK, "Orders.", swagger.JSONRef("Order"))
	if err := registry.Webhooks(webhooksFromCatalogue(map[string]bool{"order.refunded": true})...); err != nil {
		t.Fatal(err)
	}

	doc, err := swagger.Generate(router.Routes(), registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Webhooks) != len(webhookCatalogue) {
		t.Fatalf("webhooks = %d, want one per catalogue name (%d)", len(doc.Webhooks), len(webhookCatalogue))
	}
	paid := doc.Webhooks["order.paid"]
	if paid == nil || paid.Post == nil || paid.Get != nil {
		t.Fatalf("order.paid = %#v, want a POST operation", paid)
	}
	if paid.Post.RequestBody == nil || !paid.Post.RequestBody.Required {
		t.Fatalf("order.paid request body = %#v, want a required body", paid.Post.RequestBody)
	}
	if schema := paid.Post.RequestBody.Content["application/json"].Schema; schema == nil || schema.Reference() != "#/components/schemas/OrderEvent" {
		t.Fatalf("order.paid body schema = %#v, want the OrderEvent component", schema)
	}
	if !doc.Webhooks["order.refunded"].Post.Deprecated || paid.Post.Deprecated {
		t.Fatal("deprecation was not carried per webhook")
	}
	if _, isPath := doc.Paths["order.paid"]; isPath {
		t.Fatal("a webhook leaked into paths")
	}

	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"webhooks":{"order.created":{"post":`,
		`"operationId":"webhooks:order.paid"`,
		`"responses":{"2XX":{"description":"Received."}}`,
		`"data":{"$ref":"#/components/schemas/Order"}`,
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Fatalf("document misses %s: %s", fragment, encoded)
		}
	}

	again, err := swagger.GenerateJSON(router.Routes(), registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(encoded) {
		t.Fatal("generation with webhooks is not deterministic")
	}
}

func TestDocumentWithoutWebhooksHasNoWebhooksKey(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/health", func(http.ResponseWriter, *http.Request) {}).Name("health")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusNoContent, "Healthy")
	encoded, err := swagger.GenerateJSON(router.Routes(), registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"webhooks"`) {
		t.Fatalf("a document without webhooks writes the key: %s", encoded)
	}
}

func TestWebhookRegistrationIsIdempotentAndRefusesAConflict(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	first := swagger.Webhook{
		Name:      "order.paid",
		Body:      []*swagger.Media{swagger.JSON(jsonschema.Object(jsonschema.Prop("id", jsonschema.String().Required())))},
		Responses: swagger.Responses{"204": {Description: "Received."}},
	}
	if err := registry.Webhooks(first); err != nil {
		t.Fatal(err)
	}
	revision := registry.Revision()
	if err := registry.Webhooks(first); err != nil {
		t.Fatalf("registering the same webhook again: %v", err)
	}
	if registry.Revision() != revision {
		t.Fatal("an idempotent registration advanced the revision")
	}

	other := first
	other.Summary = "Changed."
	if err := registry.Webhooks(other); err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("a conflicting webhook = %v, want a conflict", err)
	}

	put := first
	put.Method = "put"
	clean := swagger.NewRegistry()
	if err := clean.Webhooks(first, put); err != nil {
		t.Fatalf("one name with two methods: %v", err)
	}
	doc, err := swagger.Generate(nil, clean, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if item := doc.Webhooks["order.paid"]; item == nil || item.Post == nil || item.Put == nil {
		t.Fatalf("order.paid = %#v, want POST and PUT", item)
	}
}

func TestWebhooksAreSnapshotsOfTheCallersValues(t *testing.T) {
	t.Parallel()

	registry := swagger.NewRegistry()
	responses := swagger.Responses{"2XX": {Description: "Received."}}
	tags := []string{"webhooks"}
	if err := registry.Webhooks(swagger.Webhook{Name: "order.paid", Tags: tags, Responses: responses}); err != nil {
		t.Fatal(err)
	}
	responses["2XX"].Description = "Mutated."
	responses["500"] = &swagger.Response{Description: "Added."}
	tags[0] = "mutated"

	doc, err := swagger.Generate(nil, registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	operation := doc.Webhooks["order.paid"].Post
	if operation.Responses["2XX"].Description != "Received." || len(operation.Responses) != 1 || operation.Tags[0] != "webhooks" {
		t.Fatalf("operation = %#v, the caller's later mutation reached the registry", operation)
	}
	if operation.RequestBody != nil {
		t.Fatal("a webhook without a body gained one")
	}
}

func TestInvalidWebhooksAreRefusedAndNothingIsRegistered(t *testing.T) {
	t.Parallel()

	received := swagger.Responses{"2XX": {Description: "Received."}}
	for name, webhook := range map[string]swagger.Webhook{
		"blank name":          {Name: " ", Responses: received},
		"padded name":         {Name: " order.paid", Responses: received},
		"unknown method":      {Name: "order.paid", Method: "CONNECT", Responses: received},
		"no response":         {Name: "order.paid"},
		"bad response key":    {Name: "order.paid", Responses: swagger.Responses{"2xx": {Description: "Received."}}},
		"nil response":        {Name: "order.paid", Responses: swagger.Responses{"200": nil}},
		"path parameter":      {Name: "order.paid", Responses: received, Parameters: []*swagger.Parameter{{Name: "id", In: swagger.ParameterInPath}}},
		"nil body media":      {Name: "order.paid", Responses: received, Body: []*swagger.Media{nil}},
		"blank tag":           {Name: "order.paid", Responses: received, Tags: []string{""}},
		"empty security":      {Name: "order.paid", Responses: received, Security: []swagger.SecurityRequirement{{}}},
		"bad extension key":   {Name: "order.paid", Responses: received, Extensions: swagger.Extensions{"vendor": json.RawMessage(`true`)}},
		"padded operation id": {Name: "order.paid", Responses: received, OperationID: " id"},
	} {
		registry := swagger.NewRegistry()
		valid := swagger.Webhook{Name: "order.created", Responses: received}
		if err := registry.Webhooks(valid, webhook); err == nil {
			t.Fatalf("%s: registration succeeded", name)
		}
		if _, err := swagger.Generate(nil, registry, swagger.Config{Title: "Test", Version: "1.0.0"}); err == nil {
			t.Fatalf("%s: generation succeeded after a refused registration", name)
		}
		// Had the valid webhook of the refused call been kept, the same
		// name with other content would now conflict.
		clean := swagger.NewRegistry()
		_ = clean.Webhooks(valid, webhook)
		changed := valid
		changed.Summary = "Registered after the refused call."
		if err := clean.Webhooks(changed); err != nil {
			t.Fatalf("%s: the refused call registered part of itself: %v", name, err)
		}
	}

	twice := swagger.NewRegistry()
	if err := twice.Webhooks(swagger.Webhook{Name: "a", Responses: received}, swagger.Webhook{Name: "a", Method: "post", Responses: received}); err == nil {
		t.Fatal("one call declared the same webhook twice")
	}
}

func TestWebhooksAreValidatedAgainstComponentsAndOperationIDs(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	route := router.Get("/api/orders", func(http.ResponseWriter, *http.Request) {}).Name("orders.index")
	received := swagger.Responses{"2XX": {Description: "Received."}}

	missing := swagger.NewRegistry()
	missing.Route(route).Response(http.StatusOK, "Orders.")
	if err := missing.Webhooks(swagger.Webhook{Name: "order.paid", Body: []*swagger.Media{swagger.JSONRef("Missing")}, Responses: received}); err != nil {
		t.Fatal(err)
	}
	if _, err := swagger.Generate(router.Routes(), missing, swagger.Config{Title: "Test", Version: "1.0.0"}); err == nil || !strings.Contains(err.Error(), `webhook "order.paid" POST`) {
		t.Fatalf("a body referencing a missing schema = %v, want an error naming the webhook", err)
	}

	clash := swagger.NewRegistry()
	clash.Route(route).Response(http.StatusOK, "Orders.")
	if err := clash.Webhooks(swagger.Webhook{Name: "order.paid", OperationID: "orders.index", Responses: received}); err != nil {
		t.Fatal(err)
	}
	if _, err := swagger.Generate(router.Routes(), clash, swagger.Config{Title: "Test", Version: "1.0.0"}); err == nil || !strings.Contains(err.Error(), `operationId "orders.index"`) {
		t.Fatalf("a webhook reusing a route operationId = %v, want an error", err)
	}

	security := swagger.NewRegistry()
	if err := security.Webhooks(swagger.Webhook{Name: "order.paid", Responses: received, Security: []swagger.SecurityRequirement{swagger.SecurityRequirementFor("signature")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := swagger.Generate(nil, security, swagger.Config{Title: "Test", Version: "1.0.0"}); err == nil || !strings.Contains(err.Error(), `missing scheme "signature"`) {
		t.Fatalf("a webhook naming a missing security scheme = %v, want an error", err)
	}
}

func TestMediaSchemaCarriesASnapshottedSchema(t *testing.T) {
	t.Parallel()

	schema, err := swagger.SchemaFromJSON([]byte(`{"type":"object","required":["type","data"],"properties":{"type":{"type":"string","enum":["order.paid"]},"data":{"$ref":"#/components/schemas/Order"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	registry := swagger.NewRegistry()
	if err := registry.Models(swagger.ModelOf[webhookOrder]("Order")); err != nil {
		t.Fatal(err)
	}
	if err := registry.Webhooks(swagger.Webhook{
		Name:      "order.paid",
		Body:      []*swagger.Media{swagger.MediaSchema("application/json", schema)},
		Responses: swagger.Responses{"2XX": {Description: "Received."}},
	}); err != nil {
		t.Fatal(err)
	}
	encoded, err := swagger.GenerateJSON(nil, registry, swagger.Config{Title: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"enum":["order.paid"]`) {
		t.Fatalf("the snapshotted body schema is missing: %s", encoded)
	}

	if err := swagger.NewRegistry().Webhooks(swagger.Webhook{
		Name:      "order.paid",
		Body:      []*swagger.Media{swagger.MediaSchema("application/json", swagger.Schema{})},
		Responses: swagger.Responses{"2XX": {Description: "Received."}},
	}); err == nil {
		t.Fatal("an empty schema was accepted as a body")
	}
}
