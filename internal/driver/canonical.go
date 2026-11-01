// MacXVM - Canonical JSON (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func Canonicalize(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeCanonical(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("the document holds trailing data")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type canonicalObject struct {
	keys   []string
	values map[string]any
}

func decodeCanonical(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("the document is not valid JSON: %w", err)
	}
	return decodeCanonicalValue(decoder, token)
}

func decodeCanonicalValue(decoder *json.Decoder, token json.Token) (any, error) {
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			obj := canonicalObject{values: map[string]any{}}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, fmt.Errorf("the document is not valid JSON: %w", err)
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("the document holds a key that is not a string")
				}
				if _, exists := obj.values[key]; exists {
					return nil, fmt.Errorf("the document holds the duplicate key %q", key)
				}
				value, err := decodeCanonical(decoder)
				if err != nil {
					return nil, err
				}
				obj.keys = append(obj.keys, key)
				obj.values[key] = value
			}
			if _, err := decoder.Token(); err != nil {
				return nil, fmt.Errorf("the document is not valid JSON: %w", err)
			}
			return obj, nil
		case '[':
			items := []any{}
			for decoder.More() {
				value, err := decodeCanonical(decoder)
				if err != nil {
					return nil, err
				}
				items = append(items, value)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, fmt.Errorf("the document is not valid JSON: %w", err)
			}
			return items, nil
		default:
			return nil, fmt.Errorf("the document holds an unexpected delimiter")
		}
	case json.Number:
		text := typed.String()
		if !isInteger(text) {
			return nil, fmt.Errorf("the number %q is not an integer", text)
		}
		return typed, nil
	case string:
		return typed, nil
	case bool:
		return typed, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("the document holds an unsupported value")
	}
}

func isInteger(text string) bool {
	if text == "" {
		return false
	}
	if strings.ContainsAny(text, ".eE") {
		return false
	}
	return true
}

func writeCanonical(out *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case canonicalObject:
		keys := append([]string(nil), typed.keys...)
		sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonicalString(out, key)
			out.WriteByte(':')
			if err := writeCanonical(out, typed.values[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
		return nil
	case []any:
		out.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
		return nil
	case json.Number:
		out.WriteString(typed.String())
		return nil
	case string:
		writeCanonicalString(out, typed)
		return nil
	case bool:
		if typed {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
		return nil
	case nil:
		out.WriteString("null")
		return nil
	default:
		return fmt.Errorf("the document holds an unsupported value")
	}
}

func writeCanonicalString(out *bytes.Buffer, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
				continue
			}
			if r == utf8.RuneError {
				out.WriteString(`�`)
				continue
			}
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
}

func lessUTF16(a, b string) bool {
	left := utf16.Encode([]rune(a))
	right := utf16.Encode([]rune(b))
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	return len(left) < len(right)
}
