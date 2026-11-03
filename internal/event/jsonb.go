package event

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Conservative nesting keeps decoding and PostgreSQL's recursive JSONB input
// within a small stack budget. The root payload object counts as one level.
const MaxPayloadDepth = 64

func validText(s string) bool   { return utf8.ValidString(s) && !strings.ContainsRune(s, 0) }
func ValidTenant(s string) bool { return len(s) >= 1 && len(s) <= 128 && validText(s) }

// encoding/json replaces malformed UTF-8 and isolated UTF-16 surrogates rather
// than rejecting them. Check the original bytes before that information is lost.
func validUnicodeJSON(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		unit, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil || unit == 0 {
			return false
		}
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

// JSONB maps numbers onto PostgreSQL numeric (131072 integer digits, 16383
// fractional digits). Inspect decimal positions; never round via float64.
func representableNumber(number string) bool {
	s := strings.TrimPrefix(number, "-")
	var exponent int64
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		v, e := strconv.ParseInt(s[i+1:], 10, 32)
		if e != nil {
			return false
		}
		exponent = v
		s = s[:i]
	}
	if exponent > 131072+int64(len(s)) || exponent < -16383-int64(len(s)) {
		return false
	}
	before, fraction := len(s), 0
	if i := strings.IndexByte(s, '.'); i >= 0 {
		before = i
		fraction = len(s) - i - 1
	}
	digits := strings.ReplaceAll(s, ".", "")
	leading := len(digits) - len(strings.TrimLeft(digits, "0"))
	whole := int64(before) + exponent - int64(leading)
	if leading == len(digits) {
		whole = 0
	}
	scale := int64(fraction) - exponent
	return whole <= 131072 && scale <= 16383
}
func validJSONB(raw []byte) bool {
	if !json.Valid(raw) || !validUnicodeJSON(raw) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	depth := 0
	for {
		token, e := d.Token()
		if e == io.EOF {
			return true
		}
		if e != nil {
			return false
		}
		switch v := token.(type) {
		case json.Delim:
			if v == '{' || v == '[' {
				depth++
				if depth > MaxPayloadDepth {
					return false
				}
			} else {
				depth--
			}
		case json.Number:
			if !representableNumber(string(v)) {
				return false
			}
		}
	}
}
