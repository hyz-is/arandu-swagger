package unit_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/hesape/routing"
	swagger "github.com/hyz-is/arandu-swagger"
)

// GenerateContract describes the HTTP surface alone: the module a route is
// registered under is left out, and every other route extension stays.
func TestGenerateContractLeavesOutTheModuleARouteIsRegisteredUnder(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter().ForModule("legacy")
	since := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	untagged := router.Get("/legacy", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		Name("legacy.index").
		Domain("legacy.example.test").
		Deprecated(since, since.AddDate(1, 0, 0))
	tagged := router.Post("/legacy", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("legacy.store")
	registry := swagger.NewRegistry()
	registry.Route(untagged).Response(http.StatusOK, "Legacy")
	registry.Route(tagged).Tags("Legacy writes").Response(http.StatusCreated, "Created")

	served, err := swagger.Generate(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	contract, err := swagger.GenerateContract(router.Routes(), registry, generationConfig())
	if err != nil {
		t.Fatalf("GenerateContract() error = %v", err)
	}

	if got := served.Paths["/legacy"].Get.Tags; !reflect.DeepEqual(got, []string{"legacy"}) {
		t.Fatalf("served untagged operation tags = %v, want the module", got)
	}
	if got := contract.Paths["/legacy"].Get.Tags; len(got) != 0 {
		t.Fatalf("contract untagged operation tags = %v, want none", got)
	}
	if got := contract.Paths["/legacy"].Post.Tags; !reflect.DeepEqual(got, []string{"Legacy writes"}) {
		t.Fatalf("contract tagged operation tags = %v, want its own tag", got)
	}

	for name, document := range map[string]*swagger.Document{"Generate": served, "GenerateContract": contract} {
		for method, operation := range map[string]*swagger.Operation{"GET": document.Paths["/legacy"].Get, "POST": document.Paths["/legacy"].Post} {
			_, hasModule := operation.Extensions["x-arandu-module"]
			if hasModule != (name == "Generate") {
				t.Errorf("%s %s carries x-arandu-module = %v", name, method, hasModule)
			}
			if _, ok := operation.Extensions["x-arandu-route-name"]; !ok {
				t.Errorf("%s %s lost x-arandu-route-name", name, method)
			}
		}
		get, err := json.Marshal(document.Paths["/legacy"].Get)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"x-arandu-domain", "x-arandu-deprecated-since", "x-arandu-sunset", `"operationId"`} {
			if !strings.Contains(string(get), fragment) {
				t.Errorf("%s operation does not contain %q: %s", name, fragment, get)
			}
		}
	}

	// Apart from the module, the two documents are the same.
	served.Paths["/legacy"].Get.Tags = nil
	delete(served.Paths["/legacy"].Get.Extensions, "x-arandu-module")
	delete(served.Paths["/legacy"].Post.Extensions, "x-arandu-module")
	servedJSON, _ := json.Marshal(served)
	contractJSON, _ := json.Marshal(contract)
	if string(servedJSON) != string(contractJSON) {
		t.Fatalf("documents differ beyond the module:\nserved   %s\ncontract %s", servedJSON, contractJSON)
	}
}

// A route table registered with no module and the same routes registered under
// one produce the same contract, so moving routes between modules does not
// churn a committed file.
func TestGenerateContractDoesNotDependOnTheRegisteringModule(t *testing.T) {
	t.Parallel()

	contractFor := func(module string) []byte {
		base := routing.NewRouter()
		router := base
		if module != "" {
			router = base.ForModule(module)
		}
		route := router.Get("/orders", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("orders.index")
		registry := swagger.NewRegistry()
		registry.Route(route).Summary("List orders").Response(http.StatusOK, "Orders")
		document, err := swagger.GenerateContract(base.Routes(), registry, generationConfig())
		if err != nil {
			t.Fatalf("GenerateContract(%q) error = %v", module, err)
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}

	standalone := contractFor("")
	for _, module := range []string{"app", "web"} {
		if got := contractFor(module); string(got) != string(standalone) {
			t.Errorf("contract under module %q =\n%s\nwant the module-less\n%s", module, got, standalone)
		}
	}
}

func TestContractServersReplaceServersOnlyInTheContract(t *testing.T) {
	t.Parallel()

	router := routing.NewRouter()
	route := router.Get("/orders", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).Name("orders.index")
	registry := swagger.NewRegistry()
	registry.Route(route).Response(http.StatusOK, "Orders")

	config := generationConfig()
	config.ContractServers = []swagger.Server{{URL: "https://api.example.test", Description: "Production"}}

	served, err := swagger.Generate(router.Routes(), registry, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(served.Servers) != 0 {
		t.Fatalf("served servers = %+v, want none", served.Servers)
	}
	contract, err := swagger.GenerateContract(router.Routes(), registry, config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(contract.Servers, config.ContractServers) {
		t.Fatalf("contract servers = %+v, want %+v", contract.Servers, config.ContractServers)
	}
	contract.Servers[0].URL = "https://mutated.example.test"
	if config.ContractServers[0].URL != "https://api.example.test" {
		t.Fatal("the contract shares the caller's ContractServers")
	}

	// Without ContractServers the contract keeps Servers.
	config.ContractServers = nil
	config.Servers = []swagger.Server{{URL: "/"}}
	contract, err = swagger.GenerateContract(router.Routes(), registry, config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(contract.Servers, config.Servers) {
		t.Fatalf("contract servers = %+v, want Servers %+v", contract.Servers, config.Servers)
	}
}

func TestContractServersAreValidatedLikeServers(t *testing.T) {
	t.Parallel()

	for name, server := range map[string]swagger.Server{
		"no URL":                   {},
		"a scheme other than http": {URL: "ftp://api.example.test"},
		"an undeclared variable":   {URL: "https://{region}.example.test"},
	} {
		config := generationConfig()
		config.Enabled = true
		config.ContractServers = []swagger.Server{{URL: "https://ok.example.test"}, server}
		err := config.Validate()
		if err == nil || !strings.Contains(err.Error(), "Config.ContractServers[1]") {
			t.Errorf("%s: Validate() error = %v, want Config.ContractServers[1] named", name, err)
		}
		router := routing.NewRouter()
		if _, err := swagger.GenerateContract(router.Routes(), swagger.NewRegistry(), config); err == nil {
			t.Errorf("%s: GenerateContract() error = nil", name)
		}
		if _, err := swagger.Generate(router.Routes(), swagger.NewRegistry(), config); err == nil {
			t.Errorf("%s: Generate() error = nil", name)
		}
	}
}
