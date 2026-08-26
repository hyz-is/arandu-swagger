package swagger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/jsonschema"
)

const (
	openAPIVersion    = "3.1.0"
	jsonSchemaDialect = "https://json-schema.org/draft/2020-12/schema"
)

var pathPlaceholderPattern = regexp.MustCompile(`\{([^{}]+)\}`)

// Generate builds and validates an OpenAPI 3.1 document from a route-table
// snapshot, an instance registry, and typed configuration. It does not start a
// server and is suitable for tests, CI, and static export by application code.
func Generate(routes []*fhttp.Route, registry *Registry, config Config) (*Document, error) {
	snapshot, err := registry.snapshot()
	if err != nil {
		return nil, err
	}

	var issues []error
	issues = append(issues, snapshot.issues...)
	issues = append(issues, validateGenerationConfig(config)...)

	document := &Document{
		OpenAPI:           openAPIVersion,
		JSONSchemaDialect: jsonSchemaDialect,
		Info: Info{
			Title:          config.Title,
			Summary:        config.Summary,
			Description:    config.Description,
			TermsOfService: config.TermsOfService,
			Contact:        cloneContact(config.Contact),
			License:        cloneLicense(config.License),
			Version:        config.Version,
		},
		Servers:      cloneServers(config.Servers),
		Paths:        make(Paths),
		Tags:         cloneTags(config.Tags),
		ExternalDocs: cloneExternalDocs(config.ExternalDocs),
	}
	if !componentsEmpty(snapshot.components) {
		components := cloneComponents(snapshot.components)
		document.Components = &components
	}

	bindings, bindingIssues := bindDocumentedRoutes(routes, snapshot.routes)
	issues = append(issues, bindingIssues...)
	operationIDs := make(map[string]string)
	methodPaths := make(map[string]string)

	for _, route := range routes {
		if route == nil || route.IsFallback() {
			continue
		}
		binding, documented := bindings[route]
		if documented && binding.draft.hidden {
			continue
		}
		if !documented && !config.IncludeUndocumented {
			continue
		}
		if excludedByPackage(route, config) {
			continue
		}

		methods, methodIssues := methodsForRoute(route, binding, documented)
		issues = append(issues, methodIssues...)
		for _, method := range methods {
			if !passesRouteFilter(route, method, config.Filter) {
				continue
			}
			path, names, pathIssues := openAPIPath(route)
			issues = append(issues, pathIssues...)
			if len(pathIssues) > 0 {
				continue
			}

			key := method + " " + path
			if previous, exists := methodPaths[key]; exists {
				issues = append(issues, fmt.Errorf("swagger: %s is declared more than once (%s and %s)", key, previous, routeLabel(route)))
				continue
			}

			operation, operationIssues := buildOperation(route, method, names, binding, documented, snapshot.components)
			issues = append(issues, operationIssues...)
			if operation == nil {
				continue
			}
			if operation.OperationID != "" {
				if previous, exists := operationIDs[operation.OperationID]; exists {
					issues = append(issues, fmt.Errorf("swagger: operationId %q is used by both %s and %s", operation.OperationID, previous, key))
				} else {
					operationIDs[operation.OperationID] = key
				}
			}
			item := document.Paths[path]
			if item == nil {
				item = &PathItem{}
				document.Paths[path] = item
			}
			if err := setPathOperation(item, method, operation); err != nil {
				issues = append(issues, fmt.Errorf("swagger: %s: %w", key, err))
				continue
			}
			methodPaths[key] = routeLabel(route)
		}
	}

	issues = append(issues, validateComponents(snapshot.components)...)
	if _, marshalErr := json.Marshal(document); marshalErr != nil {
		issues = append(issues, fmt.Errorf("swagger: document contains a value that cannot be serialized: %w", marshalErr))
	}
	if len(issues) > 0 {
		return nil, fmt.Errorf("swagger: OpenAPI generation failed:\n%w", errors.Join(sortErrors(issues)...))
	}
	return document, nil
}

// GenerateJSON builds, validates, and deterministically serializes an OpenAPI
// document as JSON.
func GenerateJSON(routes []*fhttp.Route, registry *Registry, config Config) ([]byte, error) {
	document, err := Generate(routes, registry, config)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("swagger: serialize OpenAPI document: %w", err)
	}
	if !json.Valid(encoded) {
		return nil, errors.New("swagger: serialized OpenAPI document is not valid JSON")
	}
	return encoded, nil
}

type documentedBinding struct {
	draft       operationDraft
	source      *fhttp.Route
	methodCount int
}

func bindDocumentedRoutes(routes []*fhttp.Route, documented map[*fhttp.Route]operationDraft) (map[*fhttp.Route]documentedBinding, []error) {
	bindings := make(map[*fhttp.Route]documentedBinding, len(documented))
	present := make(map[*fhttp.Route]bool, len(routes))
	byMethodPath := make(map[string][]*fhttp.Route, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		present[route] = true
		key := strings.ToUpper(route.Method) + "\x00" + route.URI()
		byMethodPath[key] = append(byMethodPath[key], route)
	}

	var issues []error
	for source, draft := range documented {
		if source == nil {
			continue
		}
		if !present[source] {
			issues = append(issues, fmt.Errorf("swagger: documented route %s is absent from the route table passed to Generate", routeLabel(source)))
			continue
		}
		methods := source.Methods()
		if isAnyRouteMethod(source.Method) && len(draft.methods) > 0 {
			methods = append([]string(nil), draft.methods...)
		}
		availableMethods := make(map[string]bool, len(methods))
		for _, method := range methods {
			availableMethods[strings.ToUpper(method)] = true
		}
		for method := range draft.operationIDs {
			if !availableMethods[strings.ToUpper(method)] {
				issues = append(issues, fmt.Errorf("swagger: operation %s defines OperationIDFor method %s, but the route does not declare it", routeLabel(source), method))
			}
		}
		methodCount := len(methods)
		if methodCount == 0 {
			methodCount = 1
		}
		binding := documentedBinding{draft: draft, source: source, methodCount: methodCount}
		bindings[source] = binding

		if isAnyRouteMethod(source.Method) {
			continue
		}
		for _, method := range methods {
			key := strings.ToUpper(method) + "\x00" + source.URI()
			for _, candidate := range byMethodPath[key] {
				if existing, exists := bindings[candidate]; exists && existing.source != source {
					issues = append(issues, fmt.Errorf("swagger: %s is documented by both %s and %s", strings.ToUpper(method)+" "+source.URI(), routeLabel(existing.source), routeLabel(source)))
					continue
				}
				bindings[candidate] = binding
			}
		}
	}
	return bindings, issues
}

func isAnyRouteMethod(method string) bool {
	method = strings.TrimSpace(method)
	return method == "" || strings.EqualFold(method, "ANY")
}

func methodsForRoute(route *fhttp.Route, binding documentedBinding, documented bool) ([]string, []error) {
	method := strings.ToUpper(strings.TrimSpace(route.Method))
	if method == "ANY" || method == "" {
		if !documented {
			return nil, nil
		}
		if len(binding.draft.methods) == 0 {
			return nil, []error{fmt.Errorf("swagger: operation %s uses ANY; call Methods with the concrete HTTP methods to document", routeLabel(route))}
		}
		var issues []error
		for _, explicit := range binding.draft.methods {
			if !supportedMethod(explicit) {
				issues = append(issues, fmt.Errorf("swagger: operation %s maps ANY to unsupported method %s", routeLabel(route), explicit))
			}
		}
		return append([]string(nil), binding.draft.methods...), issues
	}
	if !supportedMethod(method) {
		return nil, []error{fmt.Errorf("swagger: operation %s uses method %s, which OpenAPI 3.1 cannot represent", routeLabel(route), method)}
	}
	if documented && len(binding.draft.methods) > 0 {
		declared := make(map[string]bool)
		for _, candidate := range binding.source.Methods() {
			declared[strings.ToUpper(candidate)] = true
		}
		for _, explicit := range binding.draft.methods {
			if !supportedMethod(explicit) {
				return nil, []error{fmt.Errorf("swagger: operation %s selects unsupported method %s", routeLabel(binding.source), explicit)}
			}
			if !declared[explicit] {
				return nil, []error{fmt.Errorf("swagger: operation %s selects method %s, but the route does not declare it", routeLabel(binding.source), explicit)}
			}
		}
		if !containsFold(binding.draft.methods, method) {
			return nil, nil
		}
	}
	return []string{method}, nil
}

func supportedMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD", "PATCH", "TRACE":
		return true
	default:
		return false
	}
}

func openAPIPath(route *fhttp.Route) (string, []string, []error) {
	pattern := route.URI()
	label := routeLabel(route)
	if pattern == "" || pattern[0] != '/' {
		return "", nil, []error{fmt.Errorf("swagger: operation %s has non-absolute path %q", label, pattern)}
	}
	for _, match := range pathPlaceholderPattern.FindAllStringSubmatch(pattern, -1) {
		if strings.ContainsRune(match[1], ':') {
			return "", nil, []error{fmt.Errorf("swagger: operation %s has unnormalized qualified binding %q in %q; Hesape must normalize {parameter:field} before OpenAPI generation", label, match[0], pattern)}
		}
	}
	if hasMarkedPlaceholder(pattern, "?") {
		return "", nil, []error{fmt.Errorf("swagger: operation %s has optional path parameter in %q; OpenAPI path parameters are always required", label, pattern)}
	}
	if hasMarkedPlaceholder(pattern, "...") {
		return "", nil, []error{fmt.Errorf("swagger: operation %s has multi-segment wildcard in %q, which an OpenAPI path template cannot represent", label, pattern)}
	}
	if !strings.HasSuffix(pattern, "{$}") && strings.HasSuffix(pattern, "/") {
		return "", nil, []error{fmt.Errorf("swagger: operation %s uses subtree route %q; OpenAPI paths are exact, so use %q for an exact ServeMux route or hide the operation", label, pattern, pattern+"{$}")}
	}
	canonical := pattern
	if strings.HasSuffix(canonical, "{$}") {
		canonical = strings.TrimSuffix(canonical, "{$}")
		if canonical == "" {
			canonical = "/"
		}
	}

	matches := pathPlaceholderPattern.FindAllStringSubmatch(canonical, -1)
	names := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		name := match[1]
		if name == "$" {
			continue
		}
		name = strings.TrimSuffix(strings.TrimSuffix(name, "..."), "?")
		if strings.TrimSpace(name) == "" {
			return "", nil, []error{fmt.Errorf("swagger: operation %s has an empty path parameter in %q", label, pattern)}
		}
		if seen[name] {
			return "", nil, []error{fmt.Errorf("swagger: operation %s repeats path parameter %q", label, name)}
		}
		seen[name] = true
		names = append(names, name)
	}
	return canonical, names, nil
}

func hasMarkedPlaceholder(pattern, marker string) bool {
	for _, match := range pathPlaceholderPattern.FindAllStringSubmatch(pattern, -1) {
		if strings.HasSuffix(match[1], marker) {
			return true
		}
	}
	return false
}

func buildOperation(route *fhttp.Route, method string, pathNames []string, binding documentedBinding, documented bool, components Components) (*Operation, []error) {
	draft := binding.draft
	if !documented {
		draft = operationDraft{
			operationIDs: make(map[string]string),
			responses: Responses{
				"default": {Description: "Undocumented response."},
			},
		}
		binding.methodCount = 1
	}
	operation := &Operation{
		Tags:         append([]string(nil), draft.tags...),
		Summary:      draft.summary,
		Description:  draft.description,
		ExternalDocs: cloneExternalDocs(draft.externalDocs),
		Parameters:   cloneParameters(draft.parameters),
		RequestBody:  cloneRequestBody(draft.requestBody),
		Responses:    cloneResponses(draft.responses),
		Deprecated:   draft.deprecated || route.IsDeprecated(),
		Security:     cloneSecurityRequirements(draft.security),
		Extensions:   cloneExtensions(draft.extensions),
	}
	if operation.Extensions == nil {
		operation.Extensions = make(Extensions)
	}
	if len(operation.Tags) == 0 && route.Module != "" {
		operation.Tags = []string{route.Module}
	}
	operation.OperationID = operationID(route, method, binding, draft)
	if route.GetName() != "" {
		operation.Extensions["x-arandu-route-name"] = rawJSON(route.GetName())
	}
	if route.Module != "" {
		operation.Extensions["x-arandu-module"] = rawJSON(route.Module)
	}
	if domain := route.GetDomain(); domain != "" {
		operation.Extensions["x-arandu-domain"] = rawJSON(domain)
	}
	if route.IsDeprecated() {
		since, sunset := route.GetDeprecation()
		operation.Extensions["x-arandu-deprecated-since"] = rawJSON(since.UTC().Format("2006-01-02T15:04:05Z07:00"))
		operation.Extensions["x-arandu-sunset"] = rawJSON(sunset.UTC().Format("2006-01-02T15:04:05Z07:00"))
	}

	var issues []error
	operation.Parameters, issues = mergePathParameters(route, pathNames, operation.Parameters, components, issues)
	context := method + " " + route.URI()
	issues = append(issues, validateOperation(context, operation, components)...)
	return operation, issues
}

func operationID(route *fhttp.Route, method string, binding documentedBinding, draft operationDraft) string {
	if id := draft.operationIDs[method]; id != "" {
		return id
	}
	if draft.operationID != "" {
		return draft.operationID
	}
	id := route.GetName()
	if id != "" && binding.methodCount > 1 {
		return id + "." + strings.ToLower(method)
	}
	return id
}

func mergePathParameters(route *fhttp.Route, names []string, parameters []*Parameter, components Components, issues []error) ([]*Parameter, []error) {
	byName := make(map[string]*Parameter)
	var references []*Parameter
	referenceKeys := make(map[*Parameter]string)
	referencesByName := make(map[string]*Parameter)
	for _, parameter := range parameters {
		if parameter == nil {
			issues = append(issues, fmt.Errorf("swagger: operation %s contains a nil parameter", routeLabel(route)))
			continue
		}
		if parameter.Ref != "" {
			references = append(references, parameter)
			if resolved, known := resolveParameter(parameter, components.Parameters, make(map[string]bool)); known && resolved.Ref == "" {
				key := parameterIdentity(resolved)
				referenceKeys[parameter] = key
				if previous := referencesByName[key]; previous != nil {
					issues = append(issues, fmt.Errorf("swagger: operation %s references %s parameter %q more than once", routeLabel(route), resolved.In, resolved.Name))
				} else {
					referencesByName[key] = parameter
				}
			}
			continue
		}
		key := parameterIdentity(parameter)
		if _, exists := byName[key]; exists {
			issues = append(issues, fmt.Errorf("swagger: operation %s documents %s parameter %q more than once", routeLabel(route), parameter.In, parameter.Name))
			continue
		}
		byName[key] = parameter
	}
	for key, reference := range referencesByName {
		if inline := byName[key]; inline != nil {
			issues = append(issues, fmt.Errorf("swagger: operation %s declares parameter %q both inline and by reference %q", routeLabel(route), inline.Name, reference.Ref))
		}
	}

	pathSet := make(map[string]bool, len(names))
	result := make([]*Parameter, 0, len(parameters)+len(names))
	usedReferences := make(map[*Parameter]bool)
	for _, name := range names {
		pathSet[name] = true
		key := string(ParameterInPath) + "\x00" + name
		parameter := byName[key]
		if parameter == nil {
			if reference := referencesByName[key]; reference != nil {
				result = append(result, reference)
				usedReferences[reference] = true
				continue
			}
		}
		if parameter == nil {
			parameter = &Parameter{Name: name, In: ParameterInPath, Required: true}
		}
		parameter.Name = name
		parameter.In = ParameterInPath
		parameter.Required = true
		if parameter.Schema == nil {
			schema, err := inferredPathSchema(route, name)
			if err != nil {
				issues = append(issues, fmt.Errorf("swagger: operation %s path parameter %q: %w", routeLabel(route), name, err))
			} else {
				parameter.Schema = &schema
			}
		}
		result = append(result, parameter)
		delete(byName, key)
	}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Ref != "" {
			continue
		}
		if parameter.In == ParameterInPath && !pathSet[parameter.Name] {
			issues = append(issues, fmt.Errorf("swagger: operation %s documents path parameter %q, but the route path does not contain it", routeLabel(route), parameter.Name))
			continue
		}
		if parameter.In != ParameterInPath {
			result = append(result, parameter)
		}
	}
	for _, reference := range references {
		if usedReferences[reference] {
			continue
		}
		if key := referenceKeys[reference]; strings.HasPrefix(key, string(ParameterInPath)+"\x00") {
			name := strings.TrimPrefix(key, string(ParameterInPath)+"\x00")
			if !pathSet[name] {
				issues = append(issues, fmt.Errorf("swagger: operation %s references path parameter %q, but the route path does not contain it", routeLabel(route), name))
				continue
			}
		}
		result = append(result, reference)
	}
	return result, issues
}

func parameterIdentity(parameter *Parameter) string {
	name := parameter.Name
	if parameter.In == ParameterInHeader {
		name = strings.ToLower(name)
	}
	return string(parameter.In) + "\x00" + name
}

func resolveParameter(parameter *Parameter, parameters map[string]*Parameter, seen map[string]bool) (*Parameter, bool) {
	if parameter == nil {
		return nil, false
	}
	if parameter.Ref == "" {
		return parameter, true
	}
	if !strings.HasPrefix(parameter.Ref, "#/") {
		return nil, false
	}
	for name, candidate := range parameters {
		if parameter.Ref != "#/components/parameters/"+escapeJSONPointerToken(name) {
			continue
		}
		if seen[name] {
			return nil, false
		}
		seen[name] = true
		return resolveParameter(candidate, parameters, seen)
	}
	return nil, false
}

func inferredPathSchema(route *fhttp.Route, name string) (Schema, error) {
	pattern := route.GetWheres()[name]
	if pattern != "" {
		if err := validatePortablePattern(pattern); err != nil {
			return Schema{}, err
		}
		return SchemaFrom(jsonschema.String().Pattern(pattern))
	}
	return SchemaFrom(jsonschema.String())
}

func validatePortablePattern(pattern string) error {
	for _, unsupported := range []string{"(?", `\A`, `\z`, `\C`, `\Q`, `\E`, `\p{`, `\P{`, "[[:"} {
		if strings.Contains(pattern, unsupported) {
			return fmt.Errorf("Go regular expression %q is not safely portable to the ECMAScript dialect used by JSON Schema; provide an explicit parameter schema", pattern)
		}
	}
	for index := 0; index < len(pattern); index++ {
		if pattern[index] != '\\' {
			continue
		}
		index++
		if index >= len(pattern) {
			return fmt.Errorf("Go regular expression %q ends with an incomplete escape", pattern)
		}
		escaped := pattern[index]
		switch {
		case strings.ContainsRune(`dDsSwWbBfnrtv\\.^$|?*+()[]{}-/`, rune(escaped)):
			continue
		case escaped == 'x':
			if index+2 >= len(pattern) || !isHexDigit(pattern[index+1]) || !isHexDigit(pattern[index+2]) {
				return fmt.Errorf("Go regular expression %q uses a hex escape that is not portable to ECMAScript", pattern)
			}
			index += 2
		case escaped >= '0' && escaped <= '9':
			return fmt.Errorf("Go regular expression %q uses an octal or numeric escape that is not safely portable to ECMAScript", pattern)
		case escaped >= 'A' && escaped <= 'Z' || escaped >= 'a' && escaped <= 'z':
			return fmt.Errorf("Go regular expression %q uses escape \\%c, which is not safely portable to ECMAScript", pattern, escaped)
		}
	}
	return nil
}

func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func validateOperation(context string, operation *Operation, components Components) []error {
	var issues []error
	if operation.OperationID != strings.TrimSpace(operation.OperationID) {
		issues = append(issues, fmt.Errorf("swagger: %s operationId must not have surrounding whitespace", context))
	}
	if operation.ExternalDocs != nil {
		issues = append(issues, validateExternalDocs(context+" externalDocs", operation.ExternalDocs)...)
	}
	if len(operation.Responses) == 0 {
		issues = append(issues, fmt.Errorf("swagger: %s needs at least one response", context))
	}
	for _, status := range sortedKeys(operation.Responses) {
		response := operation.Responses[status]
		if response == nil {
			issues = append(issues, fmt.Errorf("swagger: %s response %s is nil", context, status))
			continue
		}
		if response.Ref != "" {
			issues = append(issues, validateComponentReference(context+" response "+status, response.Ref, "responses", components.Responses)...)
			if len(response.Headers) > 0 || len(response.Content) > 0 || len(response.Extensions) > 0 {
				issues = append(issues, fmt.Errorf("swagger: %s response %s uses $ref together with concrete response fields", context, status))
			}
			continue
		}
		if strings.TrimSpace(response.Description) == "" {
			issues = append(issues, fmt.Errorf("swagger: %s response %s needs a description", context, status))
		}
		issues = append(issues, validateContent(context+" response "+status, response.Content, components, false)...)
		for name, header := range response.Headers {
			issues = append(issues, validateHeader(context+" response "+status+" header "+name, header, components)...)
		}
	}
	if operation.RequestBody != nil {
		if operation.RequestBody.Ref != "" {
			issues = append(issues, validateComponentReference(context+" request body", operation.RequestBody.Ref, "requestBodies", components.RequestBodies)...)
			if len(operation.RequestBody.Content) > 0 || operation.RequestBody.Required || len(operation.RequestBody.Extensions) > 0 {
				issues = append(issues, fmt.Errorf("swagger: %s request body uses $ref together with concrete request-body fields", context))
			}
		} else {
			if len(operation.RequestBody.Content) == 0 {
				issues = append(issues, fmt.Errorf("swagger: %s request body needs at least one content type", context))
			}
			issues = append(issues, validateContent(context+" request body", operation.RequestBody.Content, components, true)...)
		}
	}
	for index, parameter := range operation.Parameters {
		issues = append(issues, validateParameter(fmt.Sprintf("%s parameter[%d]", context, index), parameter, components)...)
	}
	for index, requirement := range operation.Security {
		issues = append(issues, validateSecurityRequirement(fmt.Sprintf("%s security[%d]", context, index), requirement, components.SecuritySchemes)...)
	}
	return issues
}

func validateSecurityRequirement(context string, requirement SecurityRequirement, schemes map[string]*SecurityScheme) []error {
	var issues []error
	for name, scopes := range requirement {
		scheme, exists := schemes[name]
		if !exists {
			issues = append(issues, fmt.Errorf("swagger: %s names missing scheme %q", context, name))
			continue
		}
		resolved, known := resolveSecurityScheme(scheme, schemes, make(map[string]bool))
		if !known || resolved == nil {
			continue
		}
		switch resolved.Type {
		case SecuritySchemeOAuth2:
			available := make(map[string]bool)
			if resolved.Flows != nil {
				for _, flow := range []*OAuthFlow{resolved.Flows.Implicit, resolved.Flows.Password, resolved.Flows.ClientCredentials, resolved.Flows.AuthorizationCode} {
					if flow == nil {
						continue
					}
					for scope := range flow.Scopes {
						available[scope] = true
					}
				}
			}
			for _, scope := range scopes {
				if !available[scope] {
					issues = append(issues, fmt.Errorf("swagger: %s requires OAuth scope %q, but scheme %q does not declare it", context, scope, name))
				}
			}
		case SecuritySchemeOpenIDConnect:
			// OpenID Connect scopes are discovered from the provider rather than
			// declared in the Security Scheme Object.
		}
	}
	return issues
}

func resolveSecurityScheme(scheme *SecurityScheme, schemes map[string]*SecurityScheme, seen map[string]bool) (*SecurityScheme, bool) {
	if scheme == nil {
		return nil, false
	}
	if scheme.Ref == "" {
		return scheme, true
	}
	if !strings.HasPrefix(scheme.Ref, "#/") {
		return nil, false
	}
	for name, candidate := range schemes {
		if scheme.Ref != "#/components/securitySchemes/"+escapeJSONPointerToken(name) {
			continue
		}
		if seen[name] {
			return nil, false
		}
		seen[name] = true
		return resolveSecurityScheme(candidate, schemes, seen)
	}
	return nil, false
}

func validateParameter(context string, parameter *Parameter, components Components) []error {
	if parameter == nil {
		return []error{fmt.Errorf("swagger: %s is nil", context)}
	}
	if parameter.Ref != "" {
		var issues []error
		issues = append(issues, validateComponentReference(context, parameter.Ref, "parameters", components.Parameters)...)
		if parameter.Name != "" || parameter.In != "" || parameter.Required || parameter.Deprecated || parameter.AllowEmptyValue || parameter.Style != "" || parameter.Explode != nil || parameter.AllowReserved || parameter.Schema != nil || len(parameter.Example) > 0 || len(parameter.Examples) > 0 || len(parameter.Content) > 0 || len(parameter.Extensions) > 0 {
			issues = append(issues, fmt.Errorf("swagger: %s uses $ref together with concrete parameter fields", context))
		}
		return issues
	}
	var issues []error
	if strings.TrimSpace(parameter.Name) == "" {
		issues = append(issues, fmt.Errorf("swagger: %s needs a name", context))
	}
	switch parameter.In {
	case ParameterInPath, ParameterInQuery, ParameterInHeader, ParameterInCookie:
	default:
		issues = append(issues, fmt.Errorf("swagger: %s has unsupported location %q", context, parameter.In))
	}
	if parameter.In == ParameterInPath && !parameter.Required {
		issues = append(issues, fmt.Errorf("swagger: %s path parameter must be required", context))
	}
	if parameter.Schema == nil && len(parameter.Content) == 0 {
		issues = append(issues, fmt.Errorf("swagger: %s needs a schema or content", context))
	}
	if parameter.Schema != nil && len(parameter.Content) > 0 {
		issues = append(issues, fmt.Errorf("swagger: %s cannot contain both schema and content", context))
	}
	if len(parameter.Content) > 1 {
		issues = append(issues, fmt.Errorf("swagger: %s content must contain exactly one media type", context))
	}
	if parameter.Schema != nil {
		issues = append(issues, validateSchemaReference(context, *parameter.Schema, components)...)
	}
	if len(parameter.Example) > 0 && !json.Valid(parameter.Example) {
		issues = append(issues, fmt.Errorf("swagger: %s has invalid example JSON", context))
	}
	if len(parameter.Example) > 0 && len(parameter.Examples) > 0 {
		issues = append(issues, fmt.Errorf("swagger: %s cannot have both example and examples", context))
	}
	issues = append(issues, validateExamples(context, parameter.Examples, components)...)
	issues = append(issues, validateContent(context, parameter.Content, components, false)...)
	return issues
}

func validateHeader(context string, header *Header, components Components) []error {
	if header == nil {
		return []error{fmt.Errorf("swagger: %s is nil", context)}
	}
	if header.Ref != "" {
		var issues []error
		issues = append(issues, validateComponentReference(context, header.Ref, "headers", components.Headers)...)
		if header.Required || header.Deprecated || header.Style != "" || header.Explode != nil || header.Schema != nil || len(header.Example) > 0 || len(header.Examples) > 0 || len(header.Content) > 0 || len(header.Extensions) > 0 {
			issues = append(issues, fmt.Errorf("swagger: %s uses $ref together with concrete header fields", context))
		}
		return issues
	}
	var issues []error
	if header.Schema != nil {
		issues = append(issues, validateSchemaReference(context, *header.Schema, components)...)
	}
	if header.Schema == nil && len(header.Content) == 0 {
		issues = append(issues, fmt.Errorf("swagger: %s needs a schema or content", context))
	}
	if header.Schema != nil && len(header.Content) > 0 {
		issues = append(issues, fmt.Errorf("swagger: %s cannot contain both schema and content", context))
	}
	if len(header.Content) > 1 {
		issues = append(issues, fmt.Errorf("swagger: %s content must contain exactly one media type", context))
	}
	if len(header.Example) > 0 && !json.Valid(header.Example) {
		issues = append(issues, fmt.Errorf("swagger: %s has invalid example JSON", context))
	}
	if len(header.Example) > 0 && len(header.Examples) > 0 {
		issues = append(issues, fmt.Errorf("swagger: %s cannot have both example and examples", context))
	}
	issues = append(issues, validateExamples(context, header.Examples, components)...)
	issues = append(issues, validateContent(context, header.Content, components, false)...)
	return issues
}

func validateContent(context string, content Content, components Components, requestBody bool) []error {
	var issues []error
	seen := make(map[string]string, len(content))
	for contentType, media := range content {
		normalized, err := normalizeMediaType(contentType)
		if err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s has invalid content type %q", context, contentType))
		} else if previous, exists := seen[normalized]; exists {
			issues = append(issues, fmt.Errorf("swagger: %s declares equivalent content types %q and %q", context, previous, contentType))
		} else {
			seen[normalized] = contentType
		}
		if media == nil {
			issues = append(issues, fmt.Errorf("swagger: %s content type %q is nil", context, contentType))
			continue
		}
		if media.Schema != nil {
			issues = append(issues, validateSchemaReference(context+" content "+contentType, *media.Schema, components)...)
		}
		if len(media.Example) > 0 && !json.Valid(media.Example) {
			issues = append(issues, fmt.Errorf("swagger: %s content type %q has invalid example JSON", context, contentType))
		}
		if len(media.Example) > 0 && len(media.Examples) > 0 {
			issues = append(issues, fmt.Errorf("swagger: %s content type %q cannot have both example and examples", context, contentType))
		}
		issues = append(issues, validateExamples(context+" content "+contentType, media.Examples, components)...)
		issues = append(issues, validateEncodings(context+" content "+contentType, normalized, media.Schema, media.Encoding, components, requestBody)...)
	}
	return issues
}

func validateEncodings(context, contentType string, schema *Schema, encodings map[string]*Encoding, components Components, requestBody bool) []error {
	if len(encodings) == 0 {
		return nil
	}
	var issues []error
	if !requestBody {
		issues = append(issues, fmt.Errorf("swagger: %s encoding is only valid for request bodies", context))
	}
	if contentType != "" {
		mediaType, _, _ := mime.ParseMediaType(contentType)
		mediaType = strings.ToLower(mediaType)
		if mediaType != "application/x-www-form-urlencoded" && !strings.HasPrefix(mediaType, "multipart/") {
			issues = append(issues, fmt.Errorf("swagger: %s encoding requires a form or multipart media type", context))
		}
	}
	for _, property := range sortedKeys(encodings) {
		encoding := encodings[property]
		encodingContext := fmt.Sprintf("%s encoding %q", context, property)
		exists, inspectable, err := schemaDeclaresProperty(schema, property, components.Schemas, make(map[string]bool))
		if err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s cannot inspect its schema property: %w", encodingContext, err))
		} else if inspectable && !exists {
			issues = append(issues, fmt.Errorf("swagger: %s names schema property %q, but that property does not exist", encodingContext, property))
		}
		if encoding == nil {
			issues = append(issues, fmt.Errorf("swagger: %s is nil", encodingContext))
			continue
		}
		if encoding.ContentType != "" {
			if err := validateEncodingContentTypes(encoding.ContentType); err != nil {
				issues = append(issues, fmt.Errorf("swagger: %s contentType: %w", encodingContext, err))
			}
		}
		switch encoding.Style {
		case "", ParameterStyleForm, ParameterStyleSpaceDelimited, ParameterStylePipeDelimited, ParameterStyleDeepObject:
		default:
			issues = append(issues, fmt.Errorf("swagger: %s has unsupported query style %q", encodingContext, encoding.Style))
		}
		for _, name := range sortedKeys(encoding.Headers) {
			issues = append(issues, validateHeader(encodingContext+" header "+name, encoding.Headers[name], components)...)
		}
	}
	return issues
}

func schemaDeclaresProperty(schema *Schema, property string, schemas map[string]Schema, seen map[string]bool) (bool, bool, error) {
	if schema == nil || schema.IsZero() {
		return false, true, nil
	}
	var value any
	if schema.IsReference() {
		reference := schema.Reference()
		if !strings.HasPrefix(reference, "#/") {
			return false, false, nil
		}
		if seen[reference] {
			return false, false, nil
		}
		seen[reference] = true
		resolved, err := resolveLocalSchemaReference(reference, schemas)
		if err != nil {
			return false, true, err
		}
		value = resolved
	} else {
		decoded, err := decodeSchemaValue(*schema)
		if err != nil {
			return false, true, err
		}
		value = decoded
	}
	return schemaValueDeclaresProperty(value, property, schemas, seen)
}

func schemaValueDeclaresProperty(value any, property string, schemas map[string]Schema, seen map[string]bool) (bool, bool, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return false, true, nil
	}
	if properties, ok := object["properties"].(map[string]any); ok {
		if _, exists := properties[property]; exists {
			return true, true, nil
		}
	}
	inspectable := true
	if reference, ok := object["$ref"].(string); ok {
		if !strings.HasPrefix(reference, "#/") || seen[reference] {
			inspectable = false
		} else {
			seen[reference] = true
			resolved, err := resolveLocalSchemaReference(reference, schemas)
			if err != nil {
				return false, true, err
			}
			exists, known, err := schemaValueDeclaresProperty(resolved, property, schemas, seen)
			if err != nil || exists {
				return exists, known, err
			}
			inspectable = inspectable && known
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		alternatives, ok := object[keyword].([]any)
		if !ok {
			continue
		}
		for _, alternative := range alternatives {
			exists, known, err := schemaValueDeclaresProperty(alternative, property, schemas, seen)
			if err != nil || exists {
				return exists, known, err
			}
			inspectable = inspectable && known
		}
	}
	return false, inspectable, nil
}

func validateEncodingContentTypes(value string) error {
	values, err := splitMediaTypeList(value)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(values))
	for _, item := range values {
		normalized, err := normalizeMediaType(item)
		if err != nil {
			return fmt.Errorf("invalid media type %q", item)
		}
		if seen[normalized] {
			return fmt.Errorf("duplicate media type %q", item)
		}
		seen[normalized] = true
	}
	return nil
}

func splitMediaTypeList(value string) ([]string, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return nil, errors.New("media type list must not be blank or contain surrounding whitespace")
	}
	var values []string
	start := 0
	quoted := false
	escaped := false
	for index := 0; index < len(value); index++ {
		switch {
		case escaped:
			escaped = false
		case quoted && value[index] == '\\':
			escaped = true
		case value[index] == '"':
			quoted = !quoted
		case value[index] == ',' && !quoted:
			item := strings.TrimSpace(value[start:index])
			if item == "" {
				return nil, errors.New("media type list contains an empty entry")
			}
			values = append(values, item)
			start = index + 1
		}
	}
	if quoted || escaped {
		return nil, errors.New("media type list contains an unterminated quoted value")
	}
	item := strings.TrimSpace(value[start:])
	if item == "" {
		return nil, errors.New("media type list contains an empty entry")
	}
	return append(values, item), nil
}

func normalizeMediaType(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", errors.New("media type must not be blank or contain surrounding whitespace")
	}
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || !strings.Contains(mediaType, "/") {
		return "", errors.New("media type is not valid")
	}
	return strings.ToLower(mime.FormatMediaType(mediaType, parameters)), nil
}

func validateExamples(context string, examples map[string]*Example, components Components) []error {
	var issues []error
	for _, name := range sortedKeys(examples) {
		issues = append(issues, validateExample(context+" example "+name, examples[name], components)...)
	}
	return issues
}

func validateExample(context string, example *Example, components Components) []error {
	if example == nil {
		return []error{fmt.Errorf("swagger: %s is nil", context)}
	}
	if example.Ref != "" {
		var issues []error
		issues = append(issues, validateComponentReference(context, example.Ref, "examples", components.Examples)...)
		if len(example.Value) > 0 || example.ExternalValue != "" || len(example.Extensions) > 0 {
			issues = append(issues, fmt.Errorf("swagger: %s uses $ref together with concrete example fields", context))
		}
		return issues
	}
	var issues []error
	if len(example.Value) > 0 && example.ExternalValue != "" {
		issues = append(issues, fmt.Errorf("swagger: %s cannot have both value and externalValue", context))
	}
	if len(example.Value) > 0 && !json.Valid(example.Value) {
		issues = append(issues, fmt.Errorf("swagger: %s has invalid value JSON", context))
	}
	if example.ExternalValue != "" {
		if err := validateURIReference(example.ExternalValue); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s externalValue: %w", context, err))
		}
	}
	return issues
}

func validateSchemaReference(context string, schema Schema, components Components) []error {
	if schema.IsZero() {
		return []error{fmt.Errorf("swagger: %s has an empty schema", context)}
	}
	if schema.IsReference() {
		reference := schema.Reference()
		if err := validateURIReference(reference); err != nil {
			return []error{fmt.Errorf("swagger: %s has invalid $ref %q: %w", context, reference, err)}
		}
		if strings.HasPrefix(reference, "#/") {
			if err := validateLocalSchemaReference(reference, components.Schemas); err != nil {
				return []error{fmt.Errorf("swagger: %s: %w", context, err)}
			}
		}
	}
	return nil
}

func validateLocalSchemaReference(reference string, schemas map[string]Schema) error {
	target, err := resolveLocalSchemaReference(reference, schemas)
	if err != nil {
		return err
	}
	if err := validateSchemaValue(target, "$ref"); err != nil {
		return fmt.Errorf("schema target %q is not a valid schema: %w", reference, err)
	}
	return nil
}

func resolveLocalSchemaReference(reference string, schemas map[string]Schema) (any, error) {
	tokens, err := localJSONPointerTokens(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid local schema reference %q: %w", reference, err)
	}
	if len(tokens) < 3 || tokens[0] != "components" || tokens[1] != "schemas" {
		return nil, fmt.Errorf("references missing schema component %q", reference)
	}
	schema, exists := schemas[tokens[2]]
	if !exists {
		return nil, fmt.Errorf("references missing schema component %q", reference)
	}
	target, err := decodeSchemaValue(schema)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect schema component %q: %w", tokens[2], err)
	}
	for _, token := range tokens[3:] {
		switch current := target.(type) {
		case map[string]any:
			var found bool
			target, found = current[token]
			if !found {
				return nil, fmt.Errorf("references missing schema target %q", reference)
			}
		case []any:
			if !canonicalJSONArrayIndex(token) {
				return nil, fmt.Errorf("references missing schema target %q", reference)
			}
			index, err := strconv.Atoi(token)
			if err != nil || index >= len(current) {
				return nil, fmt.Errorf("references missing schema target %q", reference)
			}
			target = current[index]
		default:
			return nil, fmt.Errorf("references missing schema target %q", reference)
		}
	}
	return target, nil
}

func decodeSchemaValue(schema Schema) (any, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func canonicalJSONArrayIndex(token string) bool {
	if token == "0" {
		return true
	}
	if token == "" || token[0] < '1' || token[0] > '9' {
		return false
	}
	for index := 1; index < len(token); index++ {
		if token[index] < '0' || token[index] > '9' {
			return false
		}
	}
	return true
}

func localJSONPointerTokens(reference string) ([]string, error) {
	fragment, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return nil, fmt.Errorf("decode URI fragment: %w", err)
	}
	if !strings.HasPrefix(fragment, "/") {
		return nil, errors.New("fragment must contain an absolute JSON Pointer")
	}
	rawTokens := strings.Split(strings.TrimPrefix(fragment, "/"), "/")
	tokens := make([]string, len(rawTokens))
	for index, token := range rawTokens {
		decoded, err := unescapeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		tokens[index] = decoded
	}
	return tokens, nil
}

func unescapeJSONPointerToken(token string) (string, error) {
	var decoded strings.Builder
	decoded.Grow(len(token))
	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			decoded.WriteByte(token[index])
			continue
		}
		if index+1 >= len(token) {
			return "", fmt.Errorf("invalid JSON Pointer token %q", token)
		}
		index++
		switch token[index] {
		case '0':
			decoded.WriteByte('~')
		case '1':
			decoded.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid JSON Pointer token %q", token)
		}
	}
	return decoded.String(), nil
}

func validateComponents(components Components) []error {
	var issues []error
	for name, schema := range components.Schemas {
		issues = append(issues, validateSchemaReference("component schema "+name, schema, components)...)
	}
	for name, parameter := range components.Parameters {
		issues = append(issues, validateParameter("component parameter "+name, parameter, components)...)
	}
	for name, response := range components.Responses {
		if response == nil {
			issues = append(issues, fmt.Errorf("swagger: component response %s is nil", name))
			continue
		}
		if response.Ref != "" {
			issues = append(issues, validateComponentReference("component response "+name, response.Ref, "responses", components.Responses)...)
		}
		if response.Ref != "" && (len(response.Headers) > 0 || len(response.Content) > 0 || len(response.Extensions) > 0) {
			issues = append(issues, fmt.Errorf("swagger: component response %s uses $ref together with concrete response fields", name))
		}
		if response.Ref == "" && strings.TrimSpace(response.Description) == "" {
			issues = append(issues, fmt.Errorf("swagger: component response %s needs a description", name))
		}
		issues = append(issues, validateContent("component response "+name, response.Content, components, false)...)
		for headerName, header := range response.Headers {
			issues = append(issues, validateHeader("component response "+name+" header "+headerName, header, components)...)
		}
	}
	for name, body := range components.RequestBodies {
		if body == nil {
			issues = append(issues, fmt.Errorf("swagger: component request body %s is nil", name))
			continue
		}
		if body.Ref != "" {
			issues = append(issues, validateComponentReference("component request body "+name, body.Ref, "requestBodies", components.RequestBodies)...)
		}
		if body.Ref != "" && (len(body.Content) > 0 || body.Required || len(body.Extensions) > 0) {
			issues = append(issues, fmt.Errorf("swagger: component request body %s uses $ref together with concrete request-body fields", name))
		}
		if body.Ref == "" && len(body.Content) == 0 {
			issues = append(issues, fmt.Errorf("swagger: component request body %s needs content", name))
		}
		issues = append(issues, validateContent("component request body "+name, body.Content, components, true)...)
	}
	for name, header := range components.Headers {
		issues = append(issues, validateHeader("component header "+name, header, components)...)
	}
	for name, example := range components.Examples {
		issues = append(issues, validateExample("component example "+name, example, components)...)
	}
	for name, scheme := range components.SecuritySchemes {
		issues = append(issues, validateSecurityScheme(name, scheme, components.SecuritySchemes)...)
	}
	return issues
}

func validateSecurityScheme(name string, scheme *SecurityScheme, schemes map[string]*SecurityScheme) []error {
	context := "security scheme " + name
	if scheme == nil {
		return []error{fmt.Errorf("swagger: %s is nil", context)}
	}
	if scheme.Ref != "" {
		var issues []error
		issues = append(issues, validateComponentReference(context, scheme.Ref, "securitySchemes", schemes)...)
		if scheme.Type != "" || scheme.Name != "" || scheme.In != "" || scheme.Scheme != "" || scheme.BearerFormat != "" || scheme.Flows != nil || scheme.OpenIDConnectURL != "" || len(scheme.Extensions) > 0 {
			issues = append(issues, fmt.Errorf("swagger: %s uses $ref together with concrete security-scheme fields", context))
		}
		return issues
	}
	var issues []error
	switch scheme.Type {
	case SecuritySchemeAPIKey:
		if strings.TrimSpace(scheme.Name) == "" {
			issues = append(issues, fmt.Errorf("swagger: %s apiKey needs a name", context))
		}
		switch scheme.In {
		case APIKeyInHeader, APIKeyInQuery, APIKeyInCookie:
		default:
			issues = append(issues, fmt.Errorf("swagger: %s apiKey has invalid location %q", context, scheme.In))
		}
	case SecuritySchemeHTTP:
		if strings.TrimSpace(scheme.Scheme) == "" {
			issues = append(issues, fmt.Errorf("swagger: %s HTTP scheme is required", context))
		}
	case SecuritySchemeOAuth2:
		if scheme.Flows == nil {
			issues = append(issues, fmt.Errorf("swagger: %s OAuth2 needs flows", context))
		} else {
			issues = append(issues, validateOAuthFlows(context, scheme.Flows)...)
		}
	case SecuritySchemeOpenIDConnect:
		if err := validateAbsoluteURL(scheme.OpenIDConnectURL); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s openIdConnectUrl: %w", context, err))
		}
	case SecuritySchemeMutualTLS:
	default:
		issues = append(issues, fmt.Errorf("swagger: %s has unsupported type %q", context, scheme.Type))
	}
	return issues
}

func validateOAuthFlows(context string, flows *OAuthFlows) []error {
	if flows.Implicit == nil && flows.Password == nil && flows.ClientCredentials == nil && flows.AuthorizationCode == nil {
		return []error{fmt.Errorf("swagger: %s OAuth2 needs at least one flow", context)}
	}
	var issues []error
	if flow := flows.Implicit; flow != nil {
		issues = append(issues, validateOAuthFlow(context+" implicit", flow, true, false)...)
	}
	if flow := flows.Password; flow != nil {
		issues = append(issues, validateOAuthFlow(context+" password", flow, false, true)...)
	}
	if flow := flows.ClientCredentials; flow != nil {
		issues = append(issues, validateOAuthFlow(context+" clientCredentials", flow, false, true)...)
	}
	if flow := flows.AuthorizationCode; flow != nil {
		issues = append(issues, validateOAuthFlow(context+" authorizationCode", flow, true, true)...)
	}
	return issues
}

func validateOAuthFlow(context string, flow *OAuthFlow, authorizationRequired, tokenRequired bool) []error {
	var issues []error
	if authorizationRequired {
		if err := validateAbsoluteURL(flow.AuthorizationURL); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s authorizationUrl: %w", context, err))
		}
	}
	if tokenRequired {
		if err := validateAbsoluteURL(flow.TokenURL); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s tokenUrl: %w", context, err))
		}
	}
	if flow.RefreshURL != "" {
		if err := validateAbsoluteURL(flow.RefreshURL); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s refreshUrl: %w", context, err))
		}
	}
	if flow.Scopes == nil {
		issues = append(issues, fmt.Errorf("swagger: %s scopes must be an object, even when empty", context))
	}
	return issues
}

func validateGenerationConfig(config Config) []error {
	var issues []error
	issues = append(issues, validateOperationalConfig(config)...)
	if strings.TrimSpace(config.Title) == "" {
		issues = append(issues, errors.New("swagger: Config.Title is required to generate an OpenAPI document"))
	}
	if strings.TrimSpace(config.Version) == "" {
		issues = append(issues, errors.New("swagger: Config.Version is required to generate an OpenAPI document"))
	}
	if config.TermsOfService != "" {
		if err := validateAbsoluteURL(config.TermsOfService); err != nil {
			issues = append(issues, fmt.Errorf("swagger: Config.TermsOfService: %w", err))
		}
	}
	if config.Contact != nil {
		if config.Contact.URL != "" {
			if err := validateAbsoluteURL(config.Contact.URL); err != nil {
				issues = append(issues, fmt.Errorf("swagger: Config.Contact.URL: %w", err))
			}
		}
		if config.Contact.Email != "" {
			if _, err := mail.ParseAddress(config.Contact.Email); err != nil {
				issues = append(issues, fmt.Errorf("swagger: Config.Contact.Email %q is invalid", config.Contact.Email))
			}
		}
	}
	if config.License != nil {
		if strings.TrimSpace(config.License.Name) == "" {
			issues = append(issues, errors.New("swagger: Config.License.Name is required"))
		}
		if config.License.Identifier != "" && config.License.URL != "" {
			issues = append(issues, errors.New("swagger: Config.License cannot set both Identifier and URL in OpenAPI 3.1"))
		}
		if config.License.URL != "" {
			if err := validateAbsoluteURL(config.License.URL); err != nil {
				issues = append(issues, fmt.Errorf("swagger: Config.License.URL: %w", err))
			}
		}
	}
	for index, server := range config.Servers {
		issues = append(issues, validateServer(fmt.Sprintf("Config.Servers[%d]", index), server)...)
	}
	seenTags := make(map[string]bool)
	for index, tag := range config.Tags {
		if strings.TrimSpace(tag.Name) == "" {
			issues = append(issues, fmt.Errorf("swagger: Config.Tags[%d].Name is required", index))
		} else if seenTags[tag.Name] {
			issues = append(issues, fmt.Errorf("swagger: Config.Tags contains duplicate name %q", tag.Name))
		}
		seenTags[tag.Name] = true
		if tag.ExternalDocs != nil {
			issues = append(issues, validateExternalDocs(fmt.Sprintf("Config.Tags[%d].ExternalDocs", index), tag.ExternalDocs)...)
		}
	}
	if config.ExternalDocs != nil {
		issues = append(issues, validateExternalDocs("Config.ExternalDocs", config.ExternalDocs)...)
	}
	return issues
}

func validateServer(context string, server Server) []error {
	var issues []error
	if strings.TrimSpace(server.URL) == "" {
		issues = append(issues, fmt.Errorf("swagger: %s.URL is required", context))
	}
	used := make(map[string]bool)
	for _, match := range pathPlaceholderPattern.FindAllStringSubmatch(server.URL, -1) {
		used[match[1]] = true
	}
	for name := range used {
		variable, exists := server.Variables[name]
		if !exists {
			issues = append(issues, fmt.Errorf("swagger: %s.URL uses variable %q without a declaration", context, name))
			continue
		}
		if variable.Default == "" {
			issues = append(issues, fmt.Errorf("swagger: %s.Variables[%q].Default is required", context, name))
		}
		if len(variable.Enum) > 0 && !contains(variable.Enum, variable.Default) {
			issues = append(issues, fmt.Errorf("swagger: %s.Variables[%q].Default must appear in Enum", context, name))
		}
	}
	for name := range server.Variables {
		if !used[name] {
			issues = append(issues, fmt.Errorf("swagger: %s.Variables declares %q, but URL does not use it", context, name))
		}
	}
	resolved := server.URL
	for name, variable := range server.Variables {
		resolved = strings.ReplaceAll(resolved, "{"+name+"}", variable.Default)
	}
	if parsed, err := url.Parse(resolved); err != nil || strings.ContainsAny(resolved, "\r\n\t ") || (parsed.IsAbs() && parsed.Scheme != "http" && parsed.Scheme != "https") {
		issues = append(issues, fmt.Errorf("swagger: %s.URL %q is not a valid HTTP URL template or relative URL", context, server.URL))
	}
	return issues
}

func validateExternalDocs(context string, docs *ExternalDocumentation) []error {
	if docs == nil {
		return nil
	}
	if err := validateAbsoluteURL(docs.URL); err != nil {
		return []error{fmt.Errorf("swagger: %s.URL: %w", context, err)}
	}
	return nil
}

func validateAbsoluteURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%q must be an absolute URL", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%q must use http or https", value)
	}
	return nil
}

func setPathOperation(item *PathItem, method string, operation *Operation) error {
	var target **Operation
	switch method {
	case "GET":
		target = &item.Get
	case "PUT":
		target = &item.Put
	case "POST":
		target = &item.Post
	case "DELETE":
		target = &item.Delete
	case "OPTIONS":
		target = &item.Options
	case "HEAD":
		target = &item.Head
	case "PATCH":
		target = &item.Patch
	case "TRACE":
		target = &item.Trace
	default:
		return fmt.Errorf("unsupported method %s", method)
	}
	if *target != nil {
		return fmt.Errorf("method %s is already set on this path", method)
	}
	*target = operation
	return nil
}

func excludedByPackage(route *fhttp.Route, config Config) bool {
	path := route.URI()
	if route.Module == "swagger" {
		return true
	}
	if !config.IncludeInternal && (path == "/_arandu" || strings.HasPrefix(path, "/_arandu/")) {
		return true
	}
	effective := config.withDefaults()
	if !effective.DisableSpec && path == effective.SpecPath {
		return true
	}
	if !effective.DisableUI && (path == effective.UIPath || path == effective.UIPath+"/{$}" || path == effective.UIPath+"/swagger-initializer.js" || strings.HasPrefix(path, effective.UIPath+"/assets/")) {
		return true
	}
	return false
}

func passesRouteFilter(route *fhttp.Route, method string, filter RouteFilter) bool {
	if len(filter.IncludePrefixes) > 0 && !hasPrefix(route.URI(), filter.IncludePrefixes) {
		return false
	}
	if len(filter.IncludeNames) > 0 && !contains(filter.IncludeNames, route.GetName()) {
		return false
	}
	if len(filter.IncludeModules) > 0 && !contains(filter.IncludeModules, route.Module) {
		return false
	}
	if len(filter.IncludeMethods) > 0 && !containsFold(filter.IncludeMethods, method) {
		return false
	}
	if len(filter.IncludeDomains) > 0 && !contains(filter.IncludeDomains, route.GetDomain()) {
		return false
	}
	if filter.IncludePredicate != nil && !filter.IncludePredicate(route) {
		return false
	}
	if hasPrefix(route.URI(), filter.ExcludePrefixes) || contains(filter.ExcludeNames, route.GetName()) || contains(filter.ExcludeModules, route.Module) || containsFold(filter.ExcludeMethods, method) || contains(filter.ExcludeDomains, route.GetDomain()) {
		return false
	}
	return filter.ExcludePredicate == nil || !filter.ExcludePredicate(route)
}

func hasPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func hasLocalComponent[T any](reference, section string, components map[string]T) bool {
	if !strings.HasPrefix(reference, "#/") {
		return true
	}
	for name := range components {
		if reference == "#/components/"+section+"/"+escapeJSONPointerToken(name) {
			return true
		}
	}
	return false
}

func validateComponentReference[T any](context, reference, section string, components map[string]T) []error {
	var issues []error
	if err := validateURIReference(reference); err != nil {
		issues = append(issues, fmt.Errorf("swagger: %s has invalid $ref %q: %w", context, reference, err))
	}
	if strings.HasPrefix(reference, "#/") && !hasLocalComponent(reference, section, components) {
		issues = append(issues, fmt.Errorf("swagger: %s references missing component %q", context, reference))
	}
	return issues
}

func validateURIReference(reference string) error {
	if reference == "" {
		return errors.New("URI reference must not be empty")
	}
	if strings.TrimSpace(reference) != reference || strings.IndexFunc(reference, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return errors.New("URI reference must not contain whitespace or control characters")
	}
	if _, err := url.Parse(reference); err != nil {
		return fmt.Errorf("invalid URI reference: %w", err)
	}
	return nil
}

func componentsEmpty(components Components) bool {
	return len(components.Schemas) == 0 && len(components.Responses) == 0 && len(components.Parameters) == 0 && len(components.Examples) == 0 && len(components.RequestBodies) == 0 && len(components.Headers) == 0 && len(components.SecuritySchemes) == 0 && len(components.Callbacks) == 0 && len(components.PathItems) == 0 && len(components.Extensions) == 0
}

func rawJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func cloneContact(source *Contact) *Contact {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneLicense(source *License) *License {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneServers(source []Server) []Server {
	copy := make([]Server, len(source))
	for index, server := range source {
		copy[index] = server
		copy[index].Extensions = cloneExtensions(server.Extensions)
		if server.Variables != nil {
			copy[index].Variables = make(map[string]ServerVariable, len(server.Variables))
			for name, variable := range server.Variables {
				variable.Enum = append([]string(nil), variable.Enum...)
				variable.Extensions = cloneExtensions(variable.Extensions)
				copy[index].Variables[name] = variable
			}
		}
	}
	return copy
}

func cloneTags(source []Tag) []Tag {
	copy := make([]Tag, len(source))
	for index, tag := range source {
		copy[index] = tag
		copy[index].ExternalDocs = cloneExternalDocs(tag.ExternalDocs)
		copy[index].Extensions = cloneExtensions(tag.Extensions)
	}
	return copy
}

func canonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

func sortErrors(issues []error) []error {
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Error() < issues[j].Error() })
	return issues
}
