package swagger_test

import (
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
