package ui

import "strings"

// ResolveTranslations returns the effective translation dictionary for the given locale,
// merged with any caller-supplied overrides.
func ResolveTranslations(locale string, overrides map[string]string) map[string]string {
	var dict map[string]string
	if isPortuguese(locale) {
		dict = defaultPortugueseTranslations()
	} else {
		dict = make(map[string]string)
	}

	for k, v := range overrides {
		if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			dict[k] = v
		}
	}

	return dict
}

// Translate returns the text drawn for key, an English source string, in
// locale: the caller's override for key when it is not blank, else the
// default Portuguese text when locale is pt or pt-BR, else key itself.
func Translate(locale string, overrides map[string]string, key string) string {
	if value := overrides[key]; strings.TrimSpace(value) != "" {
		return value
	}
	if isPortuguese(locale) {
		if value, ok := defaultPortuguesePageText()[key]; ok {
			return value
		}
		if value, ok := defaultPortugueseTranslations()[key]; ok {
			return value
		}
	}
	return key
}

// isPortuguese reports whether locale selects the default Portuguese text.
func isPortuguese(locale string) bool {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized == "pt-br" || normalized == "pt"
}

// defaultPortuguesePageText is the Portuguese for the text the package's own
// markup draws around Swagger UI. It is kept apart from the Swagger UI
// dictionary because that one is sent to the browser and applied to every
// text node Swagger UI draws, where a word such as "Home" may be the name of
// an operation or a tag.
func defaultPortuguesePageText() map[string]string {
	return map[string]string{
		"Home":                  "Início",
		"API documentation":     "Documentação da API",
		"Developer integration": "Integração e desenvolvedor",
		"Toggle theme":          "Alternar tema",
	}
}

func defaultPortugueseTranslations() map[string]string {
	return map[string]string{
		// Topbar and Authorization modal
		"Authorize":                "Autorizar",
		"Authorized":               "Autorizado",
		"Available authorizations": "Autorizações disponíveis",
		"Value:":                   "Valor:",
		"Close":                    "Fechar",
		"Logout":                   "Desconectar",
		"Servers":                  "Servidores",
		"Select a definition":      "Selecione uma definição",

		// Operations and Section Headers
		"Parameters":                   "Parâmetros",
		"No parameters":                "Sem parâmetros",
		"Name":                         "Nome",
		"Description":                  "Descrição",
		"Try it out":                   "Testar",
		"Cancel":                       "Cancelar",
		"Execute":                      "Executar",
		"Clear":                        "Limpar",
		"Reset":                        "Redefinir",
		"Request body":                 "Corpo da requisição",
		"Responses":                    "Respostas",
		"Code":                         "Código",
		"Links":                        "Links",
		"No links":                     "Sem links",
		"Media type":                   "Tipo de mídia",
		"Controls Accept header":       "Controla o cabeçalho Accept",
		"Controls Content-Type header": "Controla o cabeçalho Content-Type",
		"Example Value":                "Exemplo",
		"Schema":                       "Esquema",
		"Schemas":                      "Esquemas",

		// Response execution and results
		"Server response":      "Resposta do servidor",
		"Response body":        "Corpo da resposta",
		"Response headers":     "Cabeçalhos da resposta",
		"Curl":                 "Curl",
		"Request URL":          "URL da requisição",
		"Download":             "Baixar",
		"Copy":                 "Copiar",
		"Copied":               "Copiado",
		"No response call yet": "Nenhuma chamada realizada ainda",

		// Filter and search
		"Filter by tag":        "Filtrar por tag",
		"Filter by label":      "Filtrar por rótulo",
		"Filter":               "Filtrar",
		"Loading...":           "Carregando...",
		"Failed to load spec.": "Falha ao carregar a especificação.",

		// Data types and qualifiers
		"Required": "Obrigatório",
		"required": "obrigatório",
		"optional": "opcional",
		"integer":  "inteiro",
		"string":   "string",
		"boolean":  "booleano",
		"number":   "número",
		"object":   "objeto",
		"array":    "array",
	}
}
