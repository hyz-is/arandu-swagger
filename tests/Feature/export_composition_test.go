package feature_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

// The document served over HTTP describes the application as it is composed,
// and the exported one describes its HTTP surface alone: the module a route is
// registered under is in the first and not in the second.
func TestServedDocumentKeepsTheModuleAndTheExportedOneLeavesItOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	router, module := mount(t, exportConfig(dir))
	documentOrders(router, module)

	served := request(router, swagger.DefaultSpecPath)
	if served.Code != http.StatusOK {
		t.Fatalf("specification status = %d", served.Code)
	}
	generated, err := module.GenerateJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(served.Body.Bytes(), generated) {
		t.Fatal("the served document differs from Module.GenerateJSON")
	}
	var document struct {
		Paths map[string]map[string]struct {
			Tags   []string `json:"tags"`
			Module string   `json:"x-arandu-module"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(served.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	operation := document.Paths["/api/orders"]["get"]
	if operation.Module != "orders" {
		t.Fatalf("served x-arandu-module = %q, want orders", operation.Module)
	}
	if len(operation.Tags) != 1 || operation.Tags[0] != "orders" {
		t.Fatalf("served tags = %v, want the module", operation.Tags)
	}

	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(written, []byte("x-arandu-module")) || bytes.Contains(written, []byte(`"tags"`)) {
		t.Fatalf("the exported document carries the composition:\n%s", written)
	}
	if !bytes.Contains(written, []byte(`"x-arandu-route-name": "orders.index"`)) {
		t.Fatalf("the exported document lost x-arandu-route-name:\n%s", written)
	}
}

// An application that committed its contract with Generate over a router that
// registered its routes under no module, and listed a production server in it,
// switches to openapi:export without changing a byte: the module serves no
// server and exports ContractServers.
func TestExportCheckPassesOnAContractGeneratedOverAModuleLessRouter(t *testing.T) {
	t.Parallel()

	production := []swagger.Server{{URL: "https://api.example.test"}}
	dir := t.TempDir()

	legacy := exportConfig(dir)
	legacy.Servers = production
	committed := standaloneOrders(t, legacy)
	if !bytes.Contains(committed, []byte("https://api.example.test")) {
		t.Fatal("the fixture does not list the production server")
	}
	path := filepath.Join(dir, "api.openapi.json")
	if err := os.WriteFile(path, committed, 0o644); err != nil {
		t.Fatal(err)
	}

	config := exportConfig(dir)
	config.ContractServers = production
	router, module := mount(t, config)
	documentOrders(router, module)

	if out, err := runExport(t, module, "--check"); err != nil {
		t.Fatalf("--check on the module-less contract = %v (%s)", err, out)
	}

	served := request(router, swagger.DefaultSpecPath)
	if served.Code != http.StatusOK {
		t.Fatalf("specification status = %d", served.Code)
	}
	if strings.Contains(served.Body.String(), "api.example.test") || strings.Contains(served.Body.String(), `"servers"`) {
		t.Fatalf("the served document lists a server: %s", served.Body.String())
	}
}

// ContractServers belongs to the configuration it is set on, so a document
// declared with Contract lists its own.
func TestContractDocumentsListTheirOwnContractServers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	router, module := mount(t, exportConfig(dir))
	documentOrders(router, module)

	usage := swagger.NewRegistry()
	page := router.ForModule("usage").Get("/usage", emptyHandler).Name("usage.show")
	usage.Route(page).Response(http.StatusOK, "Usage.")
	servers := []swagger.Server{{URL: "https://usage.example.test"}}
	usageConfig := swagger.Config{
		Title:           "Usage",
		Version:         "0.1.0",
		ContractName:    "usage",
		ContractDir:     dir,
		Filter:          swagger.RouteFilter{IncludeModules: []string{"usage"}},
		ContractServers: servers,
	}
	if err := module.Contract(usage, usageConfig); err != nil {
		t.Fatal(err)
	}
	// Contract snapshots the configuration.
	servers[0].URL = "https://mutated.example.test"

	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, "usage.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Servers []swagger.Server `json:"servers"`
	}
	if err := json.Unmarshal(written, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Servers) != 1 || document.Servers[0].URL != "https://usage.example.test" {
		t.Fatalf("usage servers = %+v", document.Servers)
	}
	if bytes.Contains(written, []byte("x-arandu-module")) {
		t.Fatal("a Contract document carries x-arandu-module")
	}
	api, err := os.ReadFile(filepath.Join(dir, "api.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(api, []byte(`"servers"`)) {
		t.Fatal("the module's own document lists the servers of another contract")
	}

	err = module.Contract(swagger.NewRegistry(), swagger.Config{
		Title: "Broken", Version: "1", ContractName: "broken", ContractDir: dir,
		ContractServers: []swagger.Server{{URL: "ftp://files.example.test"}},
	})
	if err == nil || !strings.Contains(err.Error(), "Config.ContractServers[0]") {
		t.Fatalf("Contract with an invalid ContractServers = %v", err)
	}
}
