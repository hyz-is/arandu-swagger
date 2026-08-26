package swagger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Extensions contains OpenAPI specification extensions keyed by an x- name.
// Values are raw JSON so an extension remains explicit without weakening the
// fixed OpenAPI objects to map[string]any.
type Extensions map[string]json.RawMessage

// Document is an OpenAPI 3.1 document.
type Document struct {
	// OpenAPI is the semantic version of the OpenAPI specification.
	OpenAPI string `json:"openapi"`
	// Info contains API identification and ownership metadata.
	Info Info `json:"info"`
	// JSONSchemaDialect is the default dialect for Schema Objects.
	JSONSchemaDialect string `json:"jsonSchemaDialect,omitempty"`
	// Servers lists the target server alternatives for the API.
	Servers []Server `json:"servers,omitempty"`
	// Paths contains the API's HTTP operations.
	Paths Paths `json:"paths"`
	// Webhooks contains incoming operations initiated by the API provider.
	Webhooks Paths `json:"webhooks,omitempty"`
	// Components contains reusable OpenAPI objects.
	Components *Components `json:"components,omitempty"`
	// Security declares the default security requirements.
	Security []SecurityRequirement `json:"security,omitzero"`
	// Tags contains document-level metadata for operation tags.
	Tags []Tag `json:"tags,omitempty"`
	// ExternalDocs links to additional documentation for the API.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	// Extensions contains specification extensions for the document.
	Extensions Extensions `json:"-"`
}

// Info describes the API exposed by an OpenAPI document.
type Info struct {
	// Title is the public API name.
	Title string `json:"title"`
	// Summary is a short description of the API.
	Summary string `json:"summary,omitempty"`
	// Description is the full API description.
	Description string `json:"description,omitempty"`
	// TermsOfService is the URL of the API terms of service.
	TermsOfService string `json:"termsOfService,omitempty"`
	// Contact identifies a point of contact for the API.
	Contact *Contact `json:"contact,omitempty"`
	// License identifies the API's license.
	License *License `json:"license,omitempty"`
	// Version is the documented API version.
	Version string `json:"version"`
	// Extensions contains specification extensions for the Info Object.
	Extensions Extensions `json:"-"`
}

// Contact identifies a point of contact for the API.
type Contact struct {
	// Name is the contact person's or organization's name.
	Name string `json:"name,omitempty"`
	// URL is a URL for the contact information.
	URL string `json:"url,omitempty"`
	// Email is the contact email address.
	Email string `json:"email,omitempty"`
	// Extensions contains specification extensions for the contact.
	Extensions Extensions `json:"-"`
}

// License identifies the license used by the API.
type License struct {
	// Name is the license name.
	Name string `json:"name"`
	// Identifier is the SPDX license expression.
	Identifier string `json:"identifier,omitempty"`
	// URL is a URL for the license text.
	URL string `json:"url,omitempty"`
	// Extensions contains specification extensions for the license.
	Extensions Extensions `json:"-"`
}

// Server describes one target server for the API.
type Server struct {
	// URL is the server URL or URL template.
	URL string `json:"url"`
	// Description explains the server's purpose.
	Description string `json:"description,omitempty"`
	// Variables defines substitutions used by the URL template.
	Variables map[string]ServerVariable `json:"variables,omitempty"`
	// Extensions contains specification extensions for the server.
	Extensions Extensions `json:"-"`
}

// ServerVariable describes a substituted value in a server URL template.
type ServerVariable struct {
	// Enum limits the variable to an allowed set of values.
	Enum []string `json:"enum,omitempty"`
	// Default is the value used when no substitution is supplied.
	Default string `json:"default"`
	// Description explains the variable's purpose.
	Description string `json:"description,omitempty"`
	// Extensions contains specification extensions for the variable.
	Extensions Extensions `json:"-"`
}

// Tag adds metadata to an operation tag.
type Tag struct {
	// Name is the tag used by operations.
	Name string `json:"name"`
	// Description explains the tagged group of operations.
	Description string `json:"description,omitempty"`
	// ExternalDocs links to additional documentation for the tag.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	// Extensions contains specification extensions for the tag.
	Extensions Extensions `json:"-"`
}

// ExternalDocumentation links an OpenAPI object to further documentation.
type ExternalDocumentation struct {
	// Description explains what the external documentation contains.
	Description string `json:"description,omitempty"`
	// URL is the target documentation URL.
	URL string `json:"url"`
	// Extensions contains specification extensions for the link.
	Extensions Extensions `json:"-"`
}

// Reference points to an OpenAPI component or another resolvable object.
type Reference struct {
	// Ref is the reference URI.
	Ref string `json:"$ref"`
	// Summary overrides the referenced object's summary where supported.
	Summary string `json:"summary,omitempty"`
	// Description overrides the referenced object's description where supported.
	Description string `json:"description,omitempty"`
	// Extensions contains specification extensions for the reference.
	Extensions Extensions `json:"-"`
}

// Paths maps absolute API paths to their operations.
type Paths map[string]*PathItem

// PathItem describes the operations available at one path.
type PathItem struct {
	// Ref is a reference to a reusable Path Item Object.
	Ref string `json:"$ref,omitempty"`
	// Summary describes every operation on this path.
	Summary string `json:"summary,omitempty"`
	// Description explains every operation on this path.
	Description string `json:"description,omitempty"`
	// Get is the HTTP GET operation.
	Get *Operation `json:"get,omitempty"`
	// Put is the HTTP PUT operation.
	Put *Operation `json:"put,omitempty"`
	// Post is the HTTP POST operation.
	Post *Operation `json:"post,omitempty"`
	// Delete is the HTTP DELETE operation.
	Delete *Operation `json:"delete,omitempty"`
	// Options is the HTTP OPTIONS operation.
	Options *Operation `json:"options,omitempty"`
	// Head is the HTTP HEAD operation.
	Head *Operation `json:"head,omitempty"`
	// Patch is the HTTP PATCH operation.
	Patch *Operation `json:"patch,omitempty"`
	// Trace is the HTTP TRACE operation.
	Trace *Operation `json:"trace,omitempty"`
	// Servers overrides the document servers for this path.
	Servers []Server `json:"servers,omitempty"`
	// Parameters applies parameters to every operation on this path.
	Parameters []*Parameter `json:"parameters,omitempty"`
	// Extensions contains specification extensions for the path item.
	Extensions Extensions `json:"-"`
}

// Operation describes one HTTP operation on a path.
type Operation struct {
	// Tags groups the operation for readers and tooling.
	Tags []string `json:"tags,omitempty"`
	// Summary is a short description of the operation.
	Summary string `json:"summary,omitempty"`
	// Description is the full operation description.
	Description string `json:"description,omitempty"`
	// ExternalDocs links to additional operation documentation.
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	// OperationID uniquely identifies the operation in this document.
	OperationID string `json:"operationId,omitempty"`
	// Parameters contains path, query, header, and cookie parameters.
	Parameters []*Parameter `json:"parameters,omitempty"`
	// RequestBody describes the optional request body.
	RequestBody *RequestBody `json:"requestBody,omitempty"`
	// Responses contains the operation's possible responses.
	Responses Responses `json:"responses"`
	// Callbacks contains callbacks initiated by this operation.
	Callbacks map[string]Callback `json:"callbacks,omitempty"`
	// Deprecated marks the operation as deprecated.
	Deprecated bool `json:"deprecated,omitempty"`
	// Security overrides the document security requirements.
	Security []SecurityRequirement `json:"security,omitzero"`
	// Servers overrides the document servers for this operation.
	Servers []Server `json:"servers,omitempty"`
	// Extensions contains specification extensions for the operation.
	Extensions Extensions `json:"-"`
}

// Callback maps a runtime expression to the path item invoked by a callback.
type Callback map[string]*PathItem

// ParameterLocation is the part of a request that carries a parameter.
type ParameterLocation string

const (
	// ParameterInQuery places a parameter in the URL query string.
	ParameterInQuery ParameterLocation = "query"
	// ParameterInHeader places a parameter in an HTTP header.
	ParameterInHeader ParameterLocation = "header"
	// ParameterInPath places a parameter in the URL path template.
	ParameterInPath ParameterLocation = "path"
	// ParameterInCookie places a parameter in an HTTP cookie.
	ParameterInCookie ParameterLocation = "cookie"
)

// ParameterStyle controls how a parameter is serialized.
type ParameterStyle string

const (
	// ParameterStyleMatrix uses semicolon-prefixed path parameters.
	ParameterStyleMatrix ParameterStyle = "matrix"
	// ParameterStyleLabel uses dot-prefixed path parameters.
	ParameterStyleLabel ParameterStyle = "label"
	// ParameterStyleForm uses conventional query or cookie serialization.
	ParameterStyleForm ParameterStyle = "form"
	// ParameterStyleSimple uses comma-separated path or header serialization.
	ParameterStyleSimple ParameterStyle = "simple"
	// ParameterStyleSpaceDelimited uses space-delimited array values.
	ParameterStyleSpaceDelimited ParameterStyle = "spaceDelimited"
	// ParameterStylePipeDelimited uses pipe-delimited array values.
	ParameterStylePipeDelimited ParameterStyle = "pipeDelimited"
	// ParameterStyleDeepObject uses bracketed query object properties.
	ParameterStyleDeepObject ParameterStyle = "deepObject"
)

// Parameter describes one operation or path parameter, or references a
// reusable parameter component.
type Parameter struct {
	// Ref is a reference to a reusable Parameter Object.
	Ref string `json:"$ref,omitempty"`
	// Name is the case-sensitive parameter name.
	Name string `json:"name,omitempty"`
	// In identifies the request location carrying the parameter.
	In ParameterLocation `json:"in,omitempty"`
	// Description explains the parameter's purpose.
	Description string `json:"description,omitempty"`
	// Required marks a parameter as mandatory.
	Required bool `json:"required,omitempty"`
	// Deprecated marks the parameter as deprecated.
	Deprecated bool `json:"deprecated,omitempty"`
	// AllowEmptyValue permits an empty query parameter value.
	AllowEmptyValue bool `json:"allowEmptyValue,omitempty"`
	// Style controls serialization of the parameter value.
	Style ParameterStyle `json:"style,omitempty"`
	// Explode controls whether composite values use separate parameters.
	Explode *bool `json:"explode,omitempty"`
	// AllowReserved permits reserved URI characters in a query value.
	AllowReserved bool `json:"allowReserved,omitempty"`
	// Schema describes the parameter value.
	Schema *Schema `json:"schema,omitempty"`
	// Example is one example parameter value.
	Example json.RawMessage `json:"example,omitempty"`
	// Examples contains named parameter examples.
	Examples map[string]*Example `json:"examples,omitempty"`
	// Content describes a complex parameter using one media type.
	Content Content `json:"content,omitempty"`
	// Extensions contains specification extensions for the parameter.
	Extensions Extensions `json:"-"`
}

// Header describes an HTTP response header, or references a reusable header
// component.
type Header struct {
	// Ref is a reference to a reusable Header Object.
	Ref string `json:"$ref,omitempty"`
	// Description explains the header's purpose.
	Description string `json:"description,omitempty"`
	// Required marks the header as mandatory.
	Required bool `json:"required,omitempty"`
	// Deprecated marks the header as deprecated.
	Deprecated bool `json:"deprecated,omitempty"`
	// Style controls serialization of the header value.
	Style ParameterStyle `json:"style,omitempty"`
	// Explode controls serialization of composite header values.
	Explode *bool `json:"explode,omitempty"`
	// Schema describes the header value.
	Schema *Schema `json:"schema,omitempty"`
	// Example is one example header value.
	Example json.RawMessage `json:"example,omitempty"`
	// Examples contains named header examples.
	Examples map[string]*Example `json:"examples,omitempty"`
	// Content describes a complex header using one media type.
	Content Content `json:"content,omitempty"`
	// Extensions contains specification extensions for the header.
	Extensions Extensions `json:"-"`
}

// RequestBody describes the body of a request, or references a reusable
// request body component.
type RequestBody struct {
	// Ref is a reference to a reusable Request Body Object.
	Ref string `json:"$ref,omitempty"`
	// Description explains the request body.
	Description string `json:"description,omitempty"`
	// Content maps accepted media types to their representations.
	Content Content `json:"content,omitempty"`
	// Required marks the request body as mandatory.
	Required bool `json:"required,omitempty"`
	// Extensions contains specification extensions for the request body.
	Extensions Extensions `json:"-"`
}

// Content maps media types to their schemas, examples, and encodings.
type Content map[string]*MediaType

// MediaType describes the representation carried by one content type.
type MediaType struct {
	// Schema describes values carried by this media type.
	Schema *Schema `json:"schema,omitempty"`
	// Example is one example media value.
	Example json.RawMessage `json:"example,omitempty"`
	// Examples contains named media examples.
	Examples map[string]*Example `json:"examples,omitempty"`
	// Encoding describes serialization of request-body properties.
	Encoding map[string]*Encoding `json:"encoding,omitempty"`
	// Extensions contains specification extensions for the media type.
	Extensions Extensions `json:"-"`
}

// Encoding describes how one request-body property is serialized.
type Encoding struct {
	// ContentType is the media type for this property.
	ContentType string `json:"contentType,omitempty"`
	// Headers adds headers associated with this property's encoding.
	Headers map[string]*Header `json:"headers,omitempty"`
	// Style controls serialization of the property value.
	Style ParameterStyle `json:"style,omitempty"`
	// Explode controls serialization of composite property values.
	Explode *bool `json:"explode,omitempty"`
	// AllowReserved permits reserved URI characters in the encoded value.
	AllowReserved bool `json:"allowReserved,omitempty"`
	// Extensions contains specification extensions for the encoding.
	Extensions Extensions `json:"-"`
}

// Responses maps HTTP status codes or default to operation responses.
type Responses map[string]*Response

// Response describes an operation response, or references a reusable
// response component.
type Response struct {
	// Ref is a reference to a reusable Response Object.
	Ref string `json:"$ref,omitempty"`
	// Description explains the response.
	Description string `json:"description,omitempty"`
	// Headers contains response header metadata.
	Headers map[string]*Header `json:"headers,omitempty"`
	// Content maps returned media types to their representations.
	Content Content `json:"content,omitempty"`
	// Extensions contains specification extensions for the response.
	Extensions Extensions `json:"-"`
}

// Example describes an example value, or references a reusable example
// component.
type Example struct {
	// Ref is a reference to a reusable Example Object.
	Ref string `json:"$ref,omitempty"`
	// Summary is a short description of the example.
	Summary string `json:"summary,omitempty"`
	// Description is the full example description.
	Description string `json:"description,omitempty"`
	// Value is the embedded example value.
	Value json.RawMessage `json:"value,omitempty"`
	// ExternalValue is a URL containing the example value.
	ExternalValue string `json:"externalValue,omitempty"`
	// Extensions contains specification extensions for the example.
	Extensions Extensions `json:"-"`
}

// Components contains reusable OpenAPI objects keyed by component name.
type Components struct {
	// Schemas contains reusable Schema Objects.
	Schemas map[string]Schema `json:"schemas,omitempty"`
	// Responses contains reusable Response Objects.
	Responses map[string]*Response `json:"responses,omitempty"`
	// Parameters contains reusable Parameter Objects.
	Parameters map[string]*Parameter `json:"parameters,omitempty"`
	// Examples contains reusable Example Objects.
	Examples map[string]*Example `json:"examples,omitempty"`
	// RequestBodies contains reusable Request Body Objects.
	RequestBodies map[string]*RequestBody `json:"requestBodies,omitempty"`
	// Headers contains reusable Header Objects.
	Headers map[string]*Header `json:"headers,omitempty"`
	// SecuritySchemes contains reusable Security Scheme Objects.
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
	// Callbacks contains reusable Callback Objects.
	Callbacks map[string]Callback `json:"callbacks,omitempty"`
	// PathItems contains reusable Path Item Objects.
	PathItems map[string]*PathItem `json:"pathItems,omitempty"`
	// Extensions contains specification extensions for components.
	Extensions Extensions `json:"-"`
}

// MarshalJSON renders the document and appends its specification extensions.
func (v Document) MarshalJSON() ([]byte, error) {
	type plain Document
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the API information and its specification extensions.
func (v Info) MarshalJSON() ([]byte, error) {
	type plain Info
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the contact and its specification extensions.
func (v Contact) MarshalJSON() ([]byte, error) {
	type plain Contact
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the license and its specification extensions.
func (v License) MarshalJSON() ([]byte, error) {
	type plain License
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the server and its specification extensions.
func (v Server) MarshalJSON() ([]byte, error) {
	type plain Server
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the server variable and its specification extensions.
func (v ServerVariable) MarshalJSON() ([]byte, error) {
	type plain ServerVariable
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the tag and its specification extensions.
func (v Tag) MarshalJSON() ([]byte, error) {
	type plain Tag
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the external documentation and its extensions.
func (v ExternalDocumentation) MarshalJSON() ([]byte, error) {
	type plain ExternalDocumentation
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the reference and its specification extensions.
func (v Reference) MarshalJSON() ([]byte, error) {
	type plain Reference
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the path item and its specification extensions.
func (v PathItem) MarshalJSON() ([]byte, error) {
	type plain PathItem
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the operation and its specification extensions.
func (v Operation) MarshalJSON() ([]byte, error) {
	type plain Operation
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the parameter and its specification extensions.
func (v Parameter) MarshalJSON() ([]byte, error) {
	type plain Parameter
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the header and its specification extensions.
func (v Header) MarshalJSON() ([]byte, error) {
	type plain Header
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the request body and its specification extensions.
func (v RequestBody) MarshalJSON() ([]byte, error) {
	type plain RequestBody
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the media type and its specification extensions.
func (v MediaType) MarshalJSON() ([]byte, error) {
	type plain MediaType
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the property encoding and its specification extensions.
func (v Encoding) MarshalJSON() ([]byte, error) {
	type plain Encoding
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the response and its specification extensions.
func (v Response) MarshalJSON() ([]byte, error) {
	type plain Response
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the example and its specification extensions.
func (v Example) MarshalJSON() ([]byte, error) {
	type plain Example
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the components and their specification extensions.
func (v Components) MarshalJSON() ([]byte, error) {
	type plain Components
	return marshalExtended(plain(v), v.Extensions)
}

func marshalExtended(value any, extensions Extensions) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(extensions) == 0 {
		return body, nil
	}
	if len(body) < 2 || body[0] != '{' || body[len(body)-1] != '}' {
		return nil, fmt.Errorf("swagger: extended OpenAPI value must marshal as an object")
	}

	keys := make([]string, 0, len(extensions))
	for key := range extensions {
		if !strings.HasPrefix(key, "x-") || len(key) == 2 {
			return nil, fmt.Errorf("swagger: extension key %q must start with x- and include a name", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out bytes.Buffer
	out.Grow(len(body) + len(extensions)*16)
	out.Write(body[:len(body)-1])
	needComma := len(body) > 2
	for _, key := range keys {
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("swagger: marshal extension key %q: %w", key, err)
		}
		encodedValue, err := json.Marshal(extensions[key])
		if err != nil {
			return nil, fmt.Errorf("swagger: marshal extension %q: %w", key, err)
		}
		if needComma {
			out.WriteByte(',')
		}
		needComma = true
		out.Write(encodedKey)
		out.WriteByte(':')
		out.Write(encodedValue)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
