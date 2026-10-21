package swagger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/console"
	hfoundation "github.com/arandu-io/hesape/foundation"
)

var _ hfoundation.Commandable = (*Module)(nil)

// ExportCommandName is the console command the module adds to the
// application's binary. It writes every document the module exports to its
// contract file, and with --check compares each file with the generated
// document instead.
const ExportCommandName = "openapi:export"

// contractSuffix ends the file name of every exported document.
const contractSuffix = ".openapi.json"

// contract is one document openapi:export writes: the registry and the
// configuration it is generated with, and the file it is written to.
type contract struct {
	registry *Registry
	config   Config
	path     string
}

// contractPath is the file a resolved configuration's document is written to.
func contractPath(config Config) string {
	return filepath.Join(config.ContractDir, config.ContractName+contractSuffix)
}

// Contract declares another document that openapi:export writes beside the
// module's own. It is generated from the same route table the module captured,
// with registry as its documentation and config as its metadata and filter,
// and written to <config.ContractDir>/<config.ContractName>.openapi.json with
// the same defaults as the module's own document.
//
// It serves an application whose route table answers more than one contract,
// such as a browser workspace and a public API: the module serves one document
// over HTTP, and the binary exports every one of them. Only the document fields
// of config are read; Enabled and the endpoint settings have no effect here.
//
// Config is validated and snapshotted when Contract is called. Title and
// Version are required, and two documents may not share a file.
func (m *Module) Contract(registry *Registry, config Config) error {
	if registry == nil {
		return errors.New("swagger: Contract requires a registry")
	}
	if issues := validateGenerationConfig(config); len(issues) > 0 {
		return issues[0]
	}
	if err := validateContractConfig(config); err != nil {
		return err
	}

	snapshot := cloneConfig(config.withDefaults())
	declared := contract{registry: registry, config: snapshot, path: contractPath(snapshot)}

	m.contractsMu.Lock()
	defer m.contractsMu.Unlock()
	for _, existing := range m.exportedContracts() {
		if filepath.Clean(existing.path) == filepath.Clean(declared.path) {
			return fmt.Errorf("swagger: a document is already exported to %s", declared.path)
		}
	}
	m.contracts = append(m.contracts, declared)
	return nil
}

// exportedContracts returns the module's own document, when its configuration
// names a Title, followed by every document declared with Contract, in
// declaration order. The caller holds contractsMu.
//
// The own document follows the Title rather than Enabled because Enabled
// decides whether the endpoints are served, and an application that serves its
// documentation only in development still commits and checks its contract in
// CI. The zero configuration names no Title and exports nothing.
func (m *Module) exportedContracts() []contract {
	all := make([]contract, 0, len(m.contracts)+1)
	if strings.TrimSpace(m.cfg.Title) != "" {
		all = append(all, contract{registry: m.registry, config: m.cfg, path: contractPath(m.cfg)})
	}
	return append(all, m.contracts...)
}

// Commands returns the openapi:export command, which the application's binary
// answers once the module is registered. It is empty when the module exports
// nothing: a configuration with no Title, such as the zero value, and no
// document declared through Contract.
//
// openapi:export writes each document as indented JSON with a trailing newline,
// so the committed file is the same bytes on every run of the same route
// table. openapi:export --check writes nothing: it fails, naming each file,
// when a committed document is missing or differs from the generated one.
func (m *Module) Commands() []console.Command {
	m.contractsMu.Lock()
	exported := len(m.exportedContracts())
	m.contractsMu.Unlock()
	if exported == 0 {
		return nil
	}

	return []console.Command{{
		Name:        ExportCommandName,
		Signature:   ExportCommandName + " {--check : Compare each committed document with the generated one and write nothing}",
		Description: "write the OpenAPI documents the route table produces to their contract files",
		Help: "Generates every OpenAPI document the swagger module exports from the route table and writes each to\n" +
			"<ContractDir>/<ContractName>.openapi.json. With --check nothing is written, and the command fails\n" +
			"naming each committed document that is missing or differs from the generated one.",
		Run: m.runExport,
	}}
}

// runExport is the body of openapi:export.
func (m *Module) runExport(_ context.Context, o *console.IO) error {
	check := o.Option("check").Bool()

	documents, err := m.exportDocuments()
	if err != nil {
		return err
	}

	if !check {
		for _, document := range documents {
			if err := writeContract(document.path, document.data); err != nil {
				return err
			}
			o.Line("wrote %s", document.path)
		}
		return nil
	}

	var stale []string
	for _, document := range documents {
		committed, err := os.ReadFile(document.path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			stale = append(stale, document.path+" is missing")
		case err != nil:
			return fmt.Errorf("swagger: read %s: %w", document.path, err)
		case !bytes.Equal(committed, document.data):
			stale = append(stale, document.path+" differs from the route table")
		default:
			o.Line("%s matches the route table", document.path)
		}
	}
	if len(stale) > 0 {
		return console.Exit(1, "%s\nrun %s and commit the result", strings.Join(stale, "\n"), ExportCommandName)
	}
	return nil
}

// exportedDocument is one generated contract, ready to be written or compared.
type exportedDocument struct {
	path string
	data []byte
}

// exportDocuments generates every exported document from one snapshot of the
// captured route table. Nothing is returned unless every document generates,
// so a failure never leaves half of the contracts rewritten.
func (m *Module) exportDocuments() ([]exportedDocument, error) {
	router, err := m.capturedRouter()
	if err != nil {
		return nil, err
	}
	routes := router.Routes()

	m.contractsMu.Lock()
	contracts := m.exportedContracts()
	m.contractsMu.Unlock()
	if len(contracts) == 0 {
		return nil, errors.New("swagger: no OpenAPI document is exported: set Config.Title or declare one with Contract")
	}

	documents := make([]exportedDocument, 0, len(contracts))
	for _, declared := range contracts {
		if err := validateContractConfig(declared.config); err != nil {
			return nil, err
		}
		data, err := contractJSON(routes, declared.registry, declared.config)
		if err != nil {
			return nil, fmt.Errorf("swagger: generate %s: %w", declared.path, err)
		}
		documents = append(documents, exportedDocument{path: declared.path, data: data})
	}
	return documents, nil
}

// contractJSON is a document in the form it is committed in: indented by two
// spaces and ending in a newline. Object keys are sorted by the encoder, so the
// same route table and registry always produce the same bytes.
func contractJSON(routes []*fhttp.Route, registry *Registry, config Config) ([]byte, error) {
	document, err := Generate(routes, registry, config)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize OpenAPI document: %w", err)
	}
	return append(encoded, '\n'), nil
}

// writeContract writes data to path, creating its directory.
func writeContract(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("swagger: create the directory of %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("swagger: write %s: %w", path, err)
	}
	return nil
}
