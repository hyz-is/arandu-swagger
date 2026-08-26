// Command static-openapi generates OpenAPI JSON without starting an HTTP
// server. Persistence is deliberately owned by the calling application.
package main

import (
	"fmt"
	"net/http"
	"os"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/swagger"
)

func main() {
	data, err := build()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func build() ([]byte, error) {
	router := fhttp.NewRouter().ForModule("health")
	registry := swagger.NewRegistry()
	route := router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).Name("health.show")
	registry.Route(route).Response(http.StatusNoContent, "Service is healthy")

	return swagger.GenerateJSON(router.Routes(), registry, swagger.Config{
		Title:   "Example API",
		Version: "1.0.0",
	})
}
