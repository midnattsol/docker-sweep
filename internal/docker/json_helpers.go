package docker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeJSONMap(data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func pickRaw(raw map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			return v
		}
	}
	return nil
}

func pickString(raw map[string]json.RawMessage, keys ...string) string {
	return jsonString(pickRaw(raw, keys...))
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%v", n)
	}

	return ""
}

func parseLabelsRaw(raw json.RawMessage) map[string]string {
	labels := make(map[string]string)
	if len(raw) == 0 || string(raw) == "null" {
		return labels
	}

	var mapLabels map[string]string
	if err := json.Unmarshal(raw, &mapLabels); err == nil {
		for k, v := range mapLabels {
			labels[k] = v
		}
		return labels
	}

	var anyLabels map[string]any
	if err := json.Unmarshal(raw, &anyLabels); err == nil {
		for k, v := range anyLabels {
			labels[k] = fmt.Sprintf("%v", v)
		}
		return labels
	}

	var strLabels string
	if err := json.Unmarshal(raw, &strLabels); err == nil {
		for k, v := range ParseLabels(strLabels) {
			labels[k] = v
		}
	}

	return labels
}

func parseNameField(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) > 0 {
			return strings.TrimSpace(arr[0])
		}
	}

	return ""
}
