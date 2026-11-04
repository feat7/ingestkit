package config

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ParseAPIKeys reads a JSON object without silently remapping duplicate decoded
// key names. Errors do not echo credentials. Callers validate their key/tenant policy.
func ParseAPIKeys(raw string) (map[string]string, error) {
	fail := errors.New("invalid or duplicate API_KEYS mapping")
	d := json.NewDecoder(strings.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fail
	}
	keys := map[string]string{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return nil, fail
		}
		key, ok := token.(string)
		if !ok {
			return nil, fail
		}
		if _, exists := keys[key]; exists {
			return nil, fail
		}
		var tenant string
		if d.Decode(&tenant) != nil {
			return nil, fail
		}
		keys[key] = tenant
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return nil, fail
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fail
	}
	return keys, nil
}
