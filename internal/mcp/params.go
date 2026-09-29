package mcp

import (
	"encoding/json"
	"strings"
)

// getStringParam extrai estritamente uma string da chave informada.
func getStringParam(args map[string]any, key string) string {
	if val, ok := args[key].(string); ok {
		return strings.TrimSpace(val)
	}
	return ""
}

// getIntParam extrai estritamente um número inteiro da chave informada.
func getIntParam(args map[string]any, key string, defaultVal int) int {
	if val, ok := args[key]; ok {
		if f, ok := val.(float64); ok && f >= 0 {
			return int(f)
		}
		if i, ok := val.(int); ok && i >= 0 {
			return i
		}
	}
	return defaultVal
}

// getBoolParam extrai um booleano da chave informada com valor default.
func getBoolParam(args map[string]any, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

// getStringSliceParam extrai um slice de strings, suportando tanto array JSON ([]any / []string) quanto string separada por vírgula.
func getStringSliceParam(args map[string]any, key string) []string {
	val, ok := args[key]
	if !ok || val == nil {
		return nil
	}
	var res []string
	switch v := val.(type) {
	case string:
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	case []string:
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				res = append(res, s)
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					res = append(res, s)
				}
			}
		}
	}
	return res
}

// parseJSON desserializa uma string JSON para o destino informado.
func parseJSON(raw string, dest any) error {
	return json.Unmarshal([]byte(raw), dest)
}

