package swagger_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
	swagger "github.com/hyz-is/arandu-swagger"
)

func ExampleNew() {
	docs, err := swagger.New(swagger.Config{
		Enabled:     true,
		Title:       "Example API",
		Version:     "1.0.0",
		Description: "HTTP API documentation.",
		UIPath:      "/docs",
		SpecPath:    "/docs/openapi.json",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(docs.Name())
	// Output: swagger
}

func ExampleModule_Route() {
	docs, err := swagger.New(swagger.Config{
		Enabled: true,
		Title:   "Example API",
		Version: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	if err := docs.Schema("User", jsonschema.Object(
		jsonschema.Prop("id", jsonschema.String().Format("uuid").Required()),
	)); err != nil {
		panic(err)
	}
	if err := docs.SecurityScheme("bearerAuth", swagger.HTTPBearer("JWT")); err != nil {
		panic(err)
	}

	router := fhttp.NewRouter().ForModule("users")
	route := router.Get("/api/users/{id}", func(http.ResponseWriter, *http.Request) {}).
		Name("users.show").
		WhereUuid("id")
	docs.Route(route).
		Summary("Show a user").
		Tags("Users").
		PathParameter("id", swagger.Description("User identifier.")).
		Response(http.StatusOK, "User found", swagger.JSONRef("User")).
		Security("bearerAuth")
	docs.Routes(router.ForModule(docs.Name()))

	document, err := docs.Generate()
	if err != nil {
		panic(err)
	}
	fmt.Println(document.OpenAPI, document.Paths["/api/users/{id}"].Get.OperationID)
	// Output: 3.1.0 users.show
}

func ExampleGenerateJSON() {
	router := fhttp.NewRouter()
	registry := swagger.NewRegistry()
	route := router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).Name("health.show")
	registry.Route(route).Response(http.StatusNoContent, "Service is healthy")

	data, err := swagger.GenerateJSON(router.Routes(), registry, swagger.Config{
		Title:   "Example API",
		Version: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(len(data) > 0)
	// Output: true
}

func ExampleSchemasFromModels() {
	type Card struct {
		Brand string  `json:"brand"`
		Last4 *string `json:"last4"`
	}
	type PaymentMethod struct {
		ID   string `json:"id"`
		Type string `json:"type" enum:"card"`
		Card Card   `json:"card"`
		Note string `json:"note,omitempty" doc:"What the customer called it."`
	}

	schemas, err := swagger.SchemasFromModels(
		swagger.ModelOf[Card]("Card"),
		swagger.ModelOf[PaymentMethod]("PaymentMethod"),
	)
	if err != nil {
		panic(err)
	}
	encoded, _ := json.Marshal(schemas["PaymentMethod"])
	fmt.Println(string(encoded))
	// Output:
	// {"type":"object","required":["id","type","card"],"properties":{"id":{"type":"string"},"type":{"type":"string","enum":["card"]},"card":{"$ref":"#/components/schemas/Card"},"note":{"type":"string","description":"What the customer called it."}}}
}

func ExampleModule_Webhooks() {
	type Order struct {
		ID          string `json:"id"`
		AmountCents int64  `json:"amount_cents"`
	}
	type OrderEvent struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data Order  `json:"data"`
	}

	docs, err := swagger.New(swagger.Config{
		Enabled: true,
		Title:   "Example API",
		Version: "1.0.0",
	})
	if err != nil {
		panic(err)
	}
	if err := docs.Models(swagger.ModelOf[Order]("Order"), swagger.ModelOf[OrderEvent]("OrderEvent")); err != nil {
		panic(err)
	}

	// The application's catalogue of the events it sends.
	events := []string{"order.created", "order.paid"}
	webhooks := make([]swagger.Webhook, 0, len(events))
	for _, name := range events {
		webhooks = append(webhooks, swagger.Webhook{
			Name:        name,
			OperationID: "webhooks:" + name,
			Summary:     "Sent when " + name + " happens.",
			Body:        []*swagger.Media{swagger.JSONRef("OrderEvent")},
			Responses:   swagger.Responses{"2XX": {Description: "Received."}},
		})
	}
	if err := docs.Webhooks(webhooks...); err != nil {
		panic(err)
	}
	docs.Routes(fhttp.NewRouter().ForModule(docs.Name()))

	document, err := docs.Generate()
	if err != nil {
		panic(err)
	}
	fmt.Println(len(document.Webhooks), document.Webhooks["order.paid"].Post.OperationID)
	// Output: 2 webhooks:order.paid
}
