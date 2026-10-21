package feature_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/console"
	hfoundation "github.com/arandu-io/hesape/foundation"
	swagger "github.com/hyz-is/arandu-swagger"
)

// runExport runs openapi:export through a console application, the way the
// application's binary dispatches it, and returns what it printed.
func runExport(t *testing.T, module *swagger.Module, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := console.NewApplication(&out, &errOut, strings.NewReader("")).Add(module.Commands()...)
	err := app.Call(context.Background(), swagger.ExportCommandName, args...)
	return out.String(), err
}

// committedForm is a document in the form openapi:export commits it.
func committedForm(t *testing.T, document *swagger.Document) []byte {
	t.Helper()
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func exportConfig(dir string) swagger.Config {
	config := enabledConfig()
	config.DisableUI = true
	config.ContractDir = dir
	config.Filter = swagger.RouteFilter{IncludePrefixes: []string{"/api/"}}
	return config
}

func documentOrders(router *fhttp.Router, module *swagger.Module) {
	orders := router.ForModule("orders").Get("/api/orders", emptyHandler).Name("orders.index")
	module.Route(orders).Summary("List orders").Response(http.StatusOK, "Orders.")
}

func TestExportWritesTheModuleDocumentToItsContractFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "contracts")
	config := exportConfig(dir)
	config.ContractName = "billing"
	router, module := mount(t, config)
	documentOrders(router, module)

	out, err := runExport(t, module)
	if err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	path := filepath.Join(dir, "billing.openapi.json")
	if !strings.Contains(out, "wrote "+path) {
		t.Fatalf("output = %q, want it to name %s", out, path)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the exported document: %v", err)
	}
	document, err := module.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if want := committedForm(t, document); !bytes.Equal(written, want) {
		t.Fatalf("exported document =\n%s\nwant\n%s", written, want)
	}
	if !bytes.HasSuffix(written, []byte("}\n")) || bytes.HasSuffix(written, []byte("\n\n")) {
		t.Fatalf("exported document does not end in exactly one newline: %q", written[len(written)-4:])
	}

	// A second run over the same route table writes the same bytes.
	if _, err := runExport(t, module); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, written) {
		t.Fatal("a second export of the same route table wrote different bytes")
	}
}

// The default is stated on the configuration: docs/contracts/api.openapi.json,
// relative to the working directory of the binary.
func TestExportDefaultsToTheDocsContractsDirectory(t *testing.T) {
	t.Chdir(t.TempDir())

	config := enabledConfig()
	config.DisableUI = true
	router, module := mount(t, config)
	documentOrders(router, module)

	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	if _, err := os.Stat(filepath.Join("docs", "contracts", "api.openapi.json")); err != nil {
		t.Fatalf("default contract file: %v", err)
	}
	if swagger.DefaultContractDir != "docs/contracts" || swagger.DefaultContractName != "api" {
		t.Fatalf("defaults = %q, %q", swagger.DefaultContractDir, swagger.DefaultContractName)
	}
}

func TestExportCheckPassesOnTheCommittedDocumentAndFailsOnAChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	router, module := mount(t, exportConfig(dir))
	documentOrders(router, module)
	path := filepath.Join(dir, "api.openapi.json")

	// Nothing committed yet: the check names the missing file.
	_, err := runExport(t, module, "--check")
	if err == nil || !strings.Contains(err.Error(), path+" is missing") {
		t.Fatalf("--check before an export = %v, want it to name %s as missing", err, path)
	}
	if console.ExitCode(err) != 1 {
		t.Fatalf("exit code = %d, want 1", console.ExitCode(err))
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("--check wrote the contract file")
	}

	if _, err := runExport(t, module); err != nil {
		t.Fatal(err)
	}
	out, err := runExport(t, module, "--check")
	if err != nil {
		t.Fatalf("--check on the exported document = %v", err)
	}
	if !strings.Contains(out, path+" matches the route table") {
		t.Fatalf("output = %q", out)
	}

	// A committed file edited by hand differs from the generated document.
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := bytes.Replace(committed, []byte("List orders"), []byte("List the orders"), 1)
	if bytes.Equal(edited, committed) {
		t.Fatal("the fixture did not change the document")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = runExport(t, module, "--check")
	if err == nil || !strings.Contains(err.Error(), path+" differs from the route table") {
		t.Fatalf("--check on an edited document = %v, want it to name %s", err, path)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, edited) {
		t.Fatal("--check rewrote the committed document")
	}

	// A route added after the export differs too: the route table is the source.
	if _, err := runExport(t, module); err != nil {
		t.Fatal(err)
	}
	refunds := router.ForModule("orders").Post("/api/refunds", emptyHandler).Name("refunds.store")
	module.Route(refunds).Response(http.StatusCreated, "Refund created.")
	if _, err := runExport(t, module, "--check"); err == nil {
		t.Fatal("--check passed after a documented route was added")
	}
}

func TestExportWritesEveryDocumentTheModuleExports(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	router, module := mount(t, exportConfig(dir))
	documentOrders(router, module)

	workspace := swagger.NewRegistry()
	page := router.ForModule("workspace").Get("/workspace", emptyHandler).Name("workspace.show")
	workspace.Route(page).Response(http.StatusOK, "The workspace page.")
	workspaceConfig := swagger.Config{
		Title:        "Workspace",
		Version:      "0.1.0",
		ContractName: "workspace",
		ContractDir:  dir,
		Filter:       swagger.RouteFilter{IncludeModules: []string{"workspace"}},
	}
	if err := module.Contract(workspace, workspaceConfig); err != nil {
		t.Fatalf("Contract() error = %v", err)
	}

	out, err := runExport(t, module)
	if err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}

	api, err := module.Generate()
	if err != nil {
		t.Fatal(err)
	}
	workspaceDocument, err := swagger.Generate(router.Routes(), workspace, workspaceConfig)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{
		"api.openapi.json":       committedForm(t, api),
		"workspace.openapi.json": committedForm(t, workspaceDocument),
	} {
		path := filepath.Join(dir, name)
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(written, want) {
			t.Fatalf("%s =\n%s\nwant\n%s", name, written, want)
		}
		if !strings.Contains(out, "wrote "+path) {
			t.Fatalf("output = %q, want it to name %s", out, path)
		}
	}
	if bytes.Contains(committedForm(t, workspaceDocument), []byte("/api/orders")) {
		t.Fatal("the workspace document carries the API routes its filter excludes")
	}

	if _, err := runExport(t, module, "--check"); err != nil {
		t.Fatalf("--check on both exported documents = %v", err)
	}
	stale := filepath.Join(dir, "workspace.openapi.json")
	if err := os.WriteFile(stale, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = runExport(t, module, "--check")
	if err == nil || !strings.Contains(err.Error(), stale) || strings.Contains(err.Error(), "api.openapi.json") {
		t.Fatalf("--check with one stale document = %v, want it to name %s and only it", err, stale)
	}
}

// A document that does not generate fails the export before anything is
// written, so the contracts are never left half rewritten.
func TestExportWritesNothingWhenADocumentFailsToGenerate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	router, module := mount(t, exportConfig(dir))
	documentOrders(router, module)

	broken := swagger.NewRegistry()
	page := router.ForModule("workspace").Get("/workspace", emptyHandler).Name("workspace.show")
	broken.Route(page).Response(http.StatusOK, "The workspace page.", swagger.JSONRef("Missing"))
	if err := module.Contract(broken, swagger.Config{Title: "Workspace", Version: "0.1.0", ContractName: "workspace", ContractDir: dir}); err != nil {
		t.Fatal(err)
	}

	_, err := runExport(t, module)
	if err == nil || !strings.Contains(err.Error(), "workspace.openapi.json") {
		t.Fatalf("openapi:export error = %v, want the failing document named", err)
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("export wrote %d file(s) although one document failed", len(entries))
	}
}

func TestContractRefusesWhatCannotBeExported(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, module := mount(t, exportConfig(dir))

	for name, declare := range map[string]func() error{
		"no registry": func() error {
			return module.Contract(nil, swagger.Config{Title: "T", Version: "1"})
		},
		"no title": func() error {
			return module.Contract(swagger.NewRegistry(), swagger.Config{Version: "1", ContractName: "other"})
		},
		"the module's own file": func() error {
			return module.Contract(swagger.NewRegistry(), swagger.Config{Title: "T", Version: "1", ContractDir: dir})
		},
		"a name with a separator": func() error {
			return module.Contract(swagger.NewRegistry(), swagger.Config{Title: "T", Version: "1", ContractName: "a/b"})
		},
	} {
		if err := declare(); err == nil {
			t.Errorf("Contract with %s = nil, want an error", name)
		}
	}
}

func TestCommandsAreTheExportCommandOnlyWhenADocumentIsExported(t *testing.T) {
	t.Parallel()

	optOut, err := swagger.New(swagger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var commandable hfoundation.Commandable = optOut
	if commands := commandable.Commands(); len(commands) != 0 {
		t.Fatalf("the zero configuration declares %d command(s), want none", len(commands))
	}

	if err := optOut.Contract(swagger.NewRegistry(), swagger.Config{Title: "Contract only", Version: "1"}); err != nil {
		t.Fatal(err)
	}
	commands := optOut.Commands()
	if len(commands) != 1 || commands[0].Name != "openapi:export" {
		t.Fatalf("commands = %+v, want openapi:export alone", commands)
	}

	enabled, err := swagger.New(enabledConfig())
	if err != nil {
		t.Fatal(err)
	}
	if commands := enabled.Commands(); len(commands) != 1 || commands[0].Name != "openapi:export" {
		t.Fatalf("enabled module commands = %+v, want openapi:export alone", commands)
	}
}

func TestExportBeforeTheRoutesAreCapturedFails(t *testing.T) {
	t.Parallel()

	module, err := swagger.New(exportConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runExport(t, module); err == nil || !strings.Contains(err.Error(), "Routes must be called") {
		t.Fatalf("openapi:export before Routes = %v", err)
	}
}

// Enabled decides whether the endpoints are served. An application that serves
// its documentation only in development still exports and checks its contract
// everywhere, so the module's own document follows its Title.
func TestExportWritesTheDocumentOfAModuleThatServesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := exportConfig(dir)
	config.Enabled = false
	router, module := mount(t, config)
	documentOrders(router, module)
	if response := request(router, swagger.DefaultSpecPath); response.Code != http.StatusNotFound {
		t.Fatalf("a disabled module answered its specification path with %d", response.Code)
	}

	if commands := module.Commands(); len(commands) != 1 {
		t.Fatalf("a disabled module with a Title declares %d command(s), want openapi:export", len(commands))
	}
	if _, err := runExport(t, module); err != nil {
		t.Fatalf("openapi:export error = %v", err)
	}
	if _, err := runExport(t, module, "--check"); err != nil {
		t.Fatalf("--check error = %v", err)
	}

	// Validation that New skips for a disabled configuration runs before the
	// export writes anything.
	config.ContractName = "../escape"
	router, module = mount(t, config)
	documentOrders(router, module)
	if _, err := runExport(t, module); err == nil || !strings.Contains(err.Error(), "Config.ContractName") {
		t.Fatalf("openapi:export with an invalid ContractName = %v", err)
	}
}
