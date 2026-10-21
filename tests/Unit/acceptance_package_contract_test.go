package unit_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	swagger "github.com/hyz-is/arandu-swagger"
)

// The only capability the package declares is the filesystem, which the
// openapi:export console command uses to write and compare the contract files.
// Serving requests still needs none.
func TestAcceptanceManifestDeclaresOnlyTheContractFilesystemCapability(t *testing.T) {
	t.Parallel()

	root := acceptanceProjectRoot(t)
	manifest, err := os.ReadFile(filepath.Join(root, "arandu.mod.toml"))
	if err != nil {
		t.Fatalf("read arandu.mod.toml: %v", err)
	}
	for capability, want := range map[string]string{
		"network":    "false",
		"filesystem": "true",
		"exec":       "false",
		"migrations": "false",
	} {
		pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(capability) + `\s*=\s*` + want + `\s*$`)
		if !pattern.Match(manifest) {
			t.Errorf("arandu.mod.toml does not declare %s = %s", capability, want)
		}
		declarations := regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(capability)+`\s*=`).FindAll(manifest, -1)
		if len(declarations) != 1 {
			t.Errorf("arandu.mod.toml declares %s %d times, want once", capability, len(declarations))
		}
	}
}

// Only contracts.go touches the filesystem, so the capability the manifest
// declares stays the export command's and does not spread to request serving.
func TestAcceptanceOnlyTheContractExportTouchesTheFilesystem(t *testing.T) {
	t.Parallel()

	root := acceptanceProjectRoot(t)
	calls := regexp.MustCompile(`\bos\.(Open|OpenFile|Create|ReadFile|WriteFile|Remove|RemoveAll|Mkdir|MkdirAll|Rename|ReadDir)\(|\bfilepath\.Walk(Dir)?\(|"io/ioutil"`)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		found := calls.Match(source)
		if name == "contracts.go" && !found {
			t.Error("contracts.go no longer touches the filesystem; set filesystem = false in arandu.mod.toml")
		}
		if name != "contracts.go" && found {
			t.Errorf("%s touches the filesystem, which only the contract export may do", name)
		}
	}
}

func TestAcceptanceDocumentationPackageHasNoPersistenceLayerOrMigrations(t *testing.T) {
	t.Parallel()

	root := acceptanceProjectRoot(t)
	for _, name := range []string{
		"model.go",
		"policy.go",
		"repository.go",
		"service.go",
		"migration.go",
		"migrations.go",
		"migrations",
	} {
		_, err := os.Stat(filepath.Join(root, name))
		if err == nil {
			t.Errorf("documentation-only package unexpectedly contains %s", name)
			continue
		}
		if !os.IsNotExist(err) {
			t.Errorf("stat %s: %v", name, err)
		}
	}

	module, err := swagger.New(swagger.Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, exists := reflect.TypeOf(module).MethodByName("Migrations"); exists {
		t.Error("Module unexpectedly exports a Migrations method")
	}
}

func TestAcceptanceVendoredSwaggerUIHashesMatchThirdPartyInventory(t *testing.T) {
	t.Parallel()

	root := acceptanceProjectRoot(t)
	expected := map[string]string{
		"internal/ui/assets/5.32.14/LICENSE":                          "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30",
		"internal/ui/assets/5.32.14/NOTICE":                           "0d20d1adef18aee3f40dd258172155521ce702ac445cb5f7b7d60ed32dad2fb2",
		"internal/ui/assets/5.32.14/swagger-ui-bundle.js":             "16d93d5cc19e54c98fb0b81157dbb3bd90780aa36b914e128a643b31e54a93f4",
		"internal/ui/assets/5.32.14/swagger-ui-bundle.js.LICENSE.txt": "c07853f3704b510a864eb56561ca4f36e0347fdaefc5176611c57575e4b5593d",
		"internal/ui/assets/5.32.14/swagger-ui.css":                   "d7f39f764aa18c7b47dd05b9af5613e373e4ac0f3557c2693d52d0abc2464d76",
	}
	thirdParty, err := os.ReadFile(filepath.Join(root, "THIRD_PARTY.md"))
	if err != nil {
		t.Fatalf("read THIRD_PARTY.md: %v", err)
	}

	for name, want := range expected {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		got := fmt.Sprintf("%x", sha256.Sum256(contents))
		if got != want {
			t.Errorf("SHA-256(%s) = %s, want %s", name, got, want)
		}
		inventoryRow := "`" + name + "` | `" + want + "`"
		if !strings.Contains(string(thirdParty), inventoryRow) {
			t.Errorf("THIRD_PARTY.md does not inventory %s with its SHA-256", name)
		}
	}
}

func acceptanceProjectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate the acceptance test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
