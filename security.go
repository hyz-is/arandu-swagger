package swagger

import "encoding/json"

// SecuritySchemeType identifies an OpenAPI security scheme kind.
type SecuritySchemeType string

const (
	// SecuritySchemeAPIKey authenticates with a named request value.
	SecuritySchemeAPIKey SecuritySchemeType = "apiKey"
	// SecuritySchemeHTTP authenticates with an HTTP authentication scheme.
	SecuritySchemeHTTP SecuritySchemeType = "http"
	// SecuritySchemeMutualTLS authenticates with mutual TLS.
	SecuritySchemeMutualTLS SecuritySchemeType = "mutualTLS"
	// SecuritySchemeOAuth2 authenticates through OAuth 2.0 flows.
	SecuritySchemeOAuth2 SecuritySchemeType = "oauth2"
	// SecuritySchemeOpenIDConnect authenticates through OpenID Connect discovery.
	SecuritySchemeOpenIDConnect SecuritySchemeType = "openIdConnect"
)

// APIKeyLocation identifies where an API key is carried in a request.
type APIKeyLocation string

const (
	// APIKeyInQuery carries an API key in the URL query string.
	APIKeyInQuery APIKeyLocation = "query"
	// APIKeyInHeader carries an API key in an HTTP header.
	APIKeyInHeader APIKeyLocation = "header"
	// APIKeyInCookie carries an API key in an HTTP cookie.
	APIKeyInCookie APIKeyLocation = "cookie"
)

// SecurityScheme describes an authentication mechanism, or references a
// reusable security scheme component.
type SecurityScheme struct {
	// Ref is a reference to a reusable Security Scheme Object.
	Ref string `json:"$ref,omitempty"`
	// Type identifies the security scheme kind.
	Type SecuritySchemeType `json:"type,omitempty"`
	// Description explains how the security scheme is used.
	Description string `json:"description,omitempty"`
	// Name is the request value carrying an API key.
	Name string `json:"name,omitempty"`
	// In is the request location carrying an API key.
	In APIKeyLocation `json:"in,omitempty"`
	// Scheme is the HTTP Authorization scheme name.
	Scheme string `json:"scheme,omitempty"`
	// BearerFormat is a documentation hint for bearer token formatting.
	BearerFormat string `json:"bearerFormat,omitempty"`
	// Flows contains the supported OAuth 2.0 flows.
	Flows *OAuthFlows `json:"flows,omitempty"`
	// OpenIDConnectURL is the OpenID Connect discovery URL.
	OpenIDConnectURL string `json:"openIdConnectUrl,omitempty"`
	// Extensions contains specification extensions for the security scheme.
	Extensions Extensions `json:"-"`
}

// OAuthFlows contains the OAuth 2.0 flows supported by a security scheme.
type OAuthFlows struct {
	// Implicit describes an OAuth 2.0 implicit flow.
	Implicit *OAuthFlow `json:"implicit,omitempty"`
	// Password describes an OAuth 2.0 resource owner password flow.
	Password *OAuthFlow `json:"password,omitempty"`
	// ClientCredentials describes an OAuth 2.0 client credentials flow.
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	// AuthorizationCode describes an OAuth 2.0 authorization code flow.
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
	// Extensions contains specification extensions for the flow collection.
	Extensions Extensions `json:"-"`
}

// OAuthFlow describes the endpoints and scopes for one OAuth 2.0 flow.
type OAuthFlow struct {
	// AuthorizationURL is the endpoint where a user grants authorization.
	AuthorizationURL string `json:"authorizationUrl,omitempty"`
	// TokenURL is the endpoint used to obtain tokens.
	TokenURL string `json:"tokenUrl,omitempty"`
	// RefreshURL is the endpoint used to refresh tokens.
	RefreshURL string `json:"refreshUrl,omitempty"`
	// Scopes maps OAuth scope names to their descriptions.
	Scopes map[string]string `json:"scopes"`
	// Extensions contains specification extensions for the flow.
	Extensions Extensions `json:"-"`
}

// SecurityRequirement maps a security scheme name to the scopes required from
// that scheme. An empty scope list is used by non-OAuth schemes.
type SecurityRequirement map[string][]string

// MarshalJSON renders absent scopes as an empty JSON array, which is the
// OpenAPI representation required for non-OAuth security schemes.
func (r SecurityRequirement) MarshalJSON() ([]byte, error) {
	normalized := make(map[string][]string, len(r))
	for name, scopes := range r {
		if scopes == nil {
			normalized[name] = []string{}
			continue
		}
		normalized[name] = scopes
	}
	return json.Marshal(normalized)
}

// HTTPBearer builds an HTTP bearer security scheme.
func HTTPBearer(format string) SecurityScheme {
	return SecurityScheme{
		Type:         SecuritySchemeHTTP,
		Scheme:       "bearer",
		BearerFormat: format,
	}
}

// HTTPBasic builds an HTTP Basic authentication security scheme.
func HTTPBasic() SecurityScheme {
	return SecurityScheme{
		Type:   SecuritySchemeHTTP,
		Scheme: "basic",
	}
}

// APIKey builds a security scheme for an API key in a query, header, or
// cookie value.
func APIKey(name string, location APIKeyLocation) SecurityScheme {
	return SecurityScheme{
		Type: SecuritySchemeAPIKey,
		Name: name,
		In:   location,
	}
}

// OAuth2 builds an OAuth 2.0 security scheme from its supported flows.
func OAuth2(flows OAuthFlows) SecurityScheme {
	return SecurityScheme{
		Type:  SecuritySchemeOAuth2,
		Flows: &flows,
	}
}

// OpenIDConnect builds an OpenID Connect security scheme from its discovery
// URL.
func OpenIDConnect(discoveryURL string) SecurityScheme {
	return SecurityScheme{
		Type:             SecuritySchemeOpenIDConnect,
		OpenIDConnectURL: discoveryURL,
	}
}

// MutualTLS builds a mutual TLS security scheme.
func MutualTLS() SecurityScheme {
	return SecurityScheme{Type: SecuritySchemeMutualTLS}
}

// SecuritySchemeRef returns a local reference to a named security scheme
// component.
func SecuritySchemeRef(name string) SecurityScheme {
	return SecurityScheme{Ref: "#/components/securitySchemes/" + escapeJSONPointerToken(name)}
}

// SecurityRequirementFor builds one requirement for a named security scheme.
func SecurityRequirementFor(name string, scopes ...string) SecurityRequirement {
	copyOfScopes := make([]string, len(scopes))
	copy(copyOfScopes, scopes)
	return SecurityRequirement{name: copyOfScopes}
}

// MarshalJSON renders the security scheme and its specification extensions.
func (v SecurityScheme) MarshalJSON() ([]byte, error) {
	type plain SecurityScheme
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the OAuth flows and their specification extensions.
func (v OAuthFlows) MarshalJSON() ([]byte, error) {
	type plain OAuthFlows
	return marshalExtended(plain(v), v.Extensions)
}

// MarshalJSON renders the OAuth flow and its specification extensions.
func (v OAuthFlow) MarshalJSON() ([]byte, error) {
	type plain OAuthFlow
	copy := plain(v)
	if copy.Scopes == nil {
		copy.Scopes = map[string]string{}
	}
	return marshalExtended(copy, v.Extensions)
}
