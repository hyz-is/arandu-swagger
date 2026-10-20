package swagger

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Webhook is one request the API sends to an endpoint its consumer
// registered: an event the provider initiates, written into the document's
// OpenAPI 3.1 webhooks section rather than its paths.
//
// A webhook is not a route of the application's router, so the route table
// cannot describe it. The values come from the application's own catalogue of
// the events it sends -- the list a subscription is validated against -- and
// are registered on the same registry as the routes, so the document has one
// source for its paths and one for its webhooks and keeps no second,
// hand-written specification:
//
//	webhooks := make([]swagger.Webhook, 0, len(events.Names()))
//	for _, name := range events.Names() {
//		webhooks = append(webhooks, swagger.Webhook{
//			Name:      name,
//			Summary:   "Sent when " + name + " happens.",
//			Body:      []*swagger.Media{swagger.JSONRef("Event")},
//			Responses: swagger.Responses{"2XX": {Description: "Received."}},
//		})
//	}
//	err := docs.Webhooks(webhooks...)
type Webhook struct {
	// Name is the key of the webhook in the document: usually the event name a
	// consumer subscribes to, such as "order.paid".
	Name string
	// Method is the HTTP method the provider sends with. Empty means POST.
	Method string
	// OperationID identifies the operation. It shares the document's
	// operationId namespace with every route operation.
	OperationID string
	// Summary is a short description of the event.
	Summary string
	// Description is the full description of the event and its delivery.
	Description string
	// Tags groups the webhook for readers and tooling.
	Tags []string
	// ExternalDocs links to additional documentation for the webhook.
	ExternalDocs *ExternalDocumentation
	// Parameters documents the headers, query or cookie values the provider
	// sends, such as a signature header. A webhook has no path parameters.
	Parameters []*Parameter
	// Body lists the representations of the request body the endpoint
	// receives. The provider always sends the body it declares, so a webhook
	// with a body has a required request body.
	Body []*Media
	// Responses are the answers the provider expects from the endpoint, keyed
	// by status code, by a status range such as "2XX", or by "default".
	Responses Responses
	// Security lists the requirements the provider satisfies when it sends.
	Security []SecurityRequirement
	// Deprecated marks an event that is kept for old subscriptions only.
	Deprecated bool
	// Extensions contains specification extensions for the operation.
	Extensions Extensions
}

// webhookKey identifies one registered webhook operation.
type webhookKey struct {
	name   string
	method string
}

func (k webhookKey) String() string { return "webhook " + strconv.Quote(k.name) + " " + k.method }

// Webhooks registers webhooks the generated document lists under its OpenAPI
// 3.1 webhooks section. Registering the same webhook again with the same
// content is idempotent; a different webhook under the same name and method
// is an error. Nothing is registered when any of them is invalid.
//
// References to components -- a JSONRef body, a ParameterRef-style parameter,
// a security scheme -- are resolved and validated by generation, together
// with the operationId uniqueness the webhooks share with the routes.
func (r *Registry) Webhooks(webhooks ...Webhook) error {
	if r == nil {
		return errors.New("swagger: cannot register webhooks on a nil Registry")
	}
	built := make(map[webhookKey]*Operation, len(webhooks))
	order := make([]webhookKey, 0, len(webhooks))
	for index := range webhooks {
		key, operation, err := buildWebhook(webhooks[index])
		if err != nil {
			return r.recordWebhookError(fmt.Errorf("swagger: webhooks[%d]: %w", index, err))
		}
		if _, repeated := built[key]; repeated {
			return r.recordWebhookError(fmt.Errorf("swagger: %s is declared more than once", key))
		}
		built[key] = operation
		order = append(order, key)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.webhooks == nil {
		r.webhooks = make(map[webhookKey]*Operation)
	}
	changed := false
	for _, key := range order {
		existing, ok := r.webhooks[key]
		if !ok {
			changed = true
			continue
		}
		if !equalJSON(existing, built[key]) {
			err := fmt.Errorf("swagger: %s is already registered with different content", key)
			r.recordLocked(err)
			return err
		}
	}
	if !changed {
		return nil
	}
	for _, key := range order {
		r.webhooks[key] = built[key]
	}
	r.revision++
	return nil
}

// Webhooks registers webhooks on this module instance. See Registry.Webhooks.
func (m *Module) Webhooks(webhooks ...Webhook) error {
	return m.registry.Webhooks(webhooks...)
}

func (r *Registry) recordWebhookError(err error) error {
	r.mu.Lock()
	r.recordLocked(err)
	r.mu.Unlock()
	return err
}

// buildWebhook validates what a webhook can be checked for without the
// components, and snapshots it as the operation generation writes.
func buildWebhook(webhook Webhook) (webhookKey, *Operation, error) {
	name := webhook.Name
	if strings.TrimSpace(name) == "" {
		return webhookKey{}, nil, errors.New("webhook name must not be blank")
	}
	if name != strings.TrimSpace(name) {
		return webhookKey{}, nil, fmt.Errorf("webhook name %q must not have surrounding whitespace", name)
	}
	method := strings.ToUpper(strings.TrimSpace(webhook.Method))
	if method == "" {
		method = "POST"
	}
	key := webhookKey{name: name, method: method}
	if !supportedMethod(method) {
		return key, nil, fmt.Errorf("%s uses unsupported method %q", key, webhook.Method)
	}
	if webhook.OperationID != strings.TrimSpace(webhook.OperationID) {
		return key, nil, fmt.Errorf("%s operationId must not have surrounding whitespace", key)
	}

	seen := make(map[string]bool, len(webhook.Tags))
	tags := make([]string, 0, len(webhook.Tags))
	for _, tag := range webhook.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return key, nil, fmt.Errorf("%s has a blank tag", key)
		}
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}

	for index, parameter := range webhook.Parameters {
		if parameter == nil {
			return key, nil, fmt.Errorf("%s parameter[%d] is nil", key, index)
		}
		if parameter.In == ParameterInPath {
			return key, nil, fmt.Errorf("%s parameter[%d] %q is a path parameter, which a webhook has no path for", key, index, parameter.Name)
		}
	}

	var body *RequestBody
	if len(webhook.Body) > 0 {
		content, _, err := buildContent(webhook.Body)
		if err != nil {
			return key, nil, fmt.Errorf("%s body: %w", key, err)
		}
		body = &RequestBody{Content: content, Required: true}
	}

	if len(webhook.Responses) == 0 {
		return key, nil, fmt.Errorf("%s needs at least one response", key)
	}
	statuses := make([]string, 0, len(webhook.Responses))
	for status := range webhook.Responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		if !validResponseKey(status) {
			return key, nil, fmt.Errorf("%s response %q must be default, an HTTP status from 100 through 599, or a range from 1XX through 5XX", key, status)
		}
		if webhook.Responses[status] == nil {
			return key, nil, fmt.Errorf("%s response %s is nil", key, status)
		}
	}

	for index, requirement := range webhook.Security {
		if len(requirement) == 0 {
			return key, nil, fmt.Errorf("%s security[%d] must name at least one scheme", key, index)
		}
	}
	for extension := range webhook.Extensions {
		if !strings.HasPrefix(extension, "x-") || len(extension) == 2 {
			return key, nil, fmt.Errorf("%s extension key %q must start with x- and include a name", key, extension)
		}
	}

	operation := &Operation{
		Tags:         tags,
		Summary:      webhook.Summary,
		Description:  webhook.Description,
		ExternalDocs: cloneExternalDocs(webhook.ExternalDocs),
		OperationID:  webhook.OperationID,
		Parameters:   cloneParameters(webhook.Parameters),
		RequestBody:  body,
		Responses:    cloneResponses(webhook.Responses),
		Deprecated:   webhook.Deprecated,
		Security:     cloneSecurityRequirements(webhook.Security),
		Extensions:   cloneExtensions(webhook.Extensions),
	}
	if len(operation.Tags) == 0 {
		operation.Tags = nil
	}
	return key, operation, nil
}

// validResponseKey reports whether status is a key the OpenAPI Responses
// Object accepts.
func validResponseKey(status string) bool {
	if status == "default" {
		return true
	}
	if len(status) != 3 {
		return false
	}
	if status[0] >= '1' && status[0] <= '5' && status[1:] == "XX" {
		return true
	}
	code, err := strconv.Atoi(status)
	return err == nil && code >= 100 && code <= 599
}

// generateWebhooks writes the registered webhooks as the document's webhooks
// section, validating them against the components and the operationIds the
// routes already took.
func generateWebhooks(webhooks map[webhookKey]*Operation, components Components, operationIDs map[string]string) (Paths, []error) {
	if len(webhooks) == 0 {
		return nil, nil
	}
	keys := make([]webhookKey, 0, len(webhooks))
	for key := range webhooks {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].method < keys[j].method
	})

	var issues []error
	out := make(Paths, len(keys))
	for _, key := range keys {
		operation := cloneOperation(webhooks[key])
		context := key.String()
		issues = append(issues, validateOperation(context, operation, components)...)
		if operation.OperationID != "" {
			if previous, exists := operationIDs[operation.OperationID]; exists {
				issues = append(issues, fmt.Errorf("swagger: operationId %q is used by both %s and %s", operation.OperationID, previous, context))
			} else {
				operationIDs[operation.OperationID] = context
			}
		}
		item := out[key.name]
		if item == nil {
			item = &PathItem{}
			out[key.name] = item
		}
		if err := setPathOperation(item, key.method, operation); err != nil {
			issues = append(issues, fmt.Errorf("swagger: %s: %w", context, err))
		}
	}
	return out, issues
}

// cloneOperation copies a webhook operation. Callbacks are not carried: a
// Webhook value declares none.
func cloneOperation(source *Operation) *Operation {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Tags = append([]string(nil), source.Tags...)
	if len(copy.Tags) == 0 {
		copy.Tags = nil
	}
	copy.ExternalDocs = cloneExternalDocs(source.ExternalDocs)
	copy.Parameters = cloneParameters(source.Parameters)
	copy.RequestBody = cloneRequestBody(source.RequestBody)
	copy.Responses = cloneResponses(source.Responses)
	copy.Callbacks = nil
	copy.Security = cloneSecurityRequirements(source.Security)
	copy.Servers = cloneServers(source.Servers)
	copy.Extensions = cloneExtensions(source.Extensions)
	return &copy
}

func cloneWebhooks(source map[webhookKey]*Operation) map[webhookKey]*Operation {
	if len(source) == 0 {
		return nil
	}
	copy := make(map[webhookKey]*Operation, len(source))
	for key, operation := range source {
		copy[key] = cloneOperation(operation)
	}
	return copy
}
