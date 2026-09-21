package ui

import "strings"

// ResolveTranslations returns the effective translation dictionary for the given locale,
// merged with any caller-supplied overrides.
func ResolveTranslations(locale string, overrides map[string]string) map[string]string {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	normalized = strings.ReplaceAll(normalized, "_", "-")

	var dict map[string]string
	if normalized == "pt-br" || normalized == "pt" {
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
