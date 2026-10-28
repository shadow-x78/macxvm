// MacXVM - Property List Parser (MIT)
// https://github.com/shadow-x78/macxvm

// Package kextmap reads the IOKit personalities of the macOS kernel
// extensions that ship inside a guest. It is the basis of the reverse
// engineering that tells MacXVM which virtual devices macOS can drive
// natively and which ones it will ignore.
package kextmap

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Value is a decoded property list value: map[string]any, []any, string,
// int64, float64, bool or []byte.
type Value any

// Plist is a decoded property list document.
type Plist struct {
	Root Value
}

// Dict returns the root value as a dictionary.
func (p Plist) Dict() (map[string]any, bool) {
	d, ok := p.Root.(map[string]any)
	return d, ok
}

// ParsePlistXML decodes an XML property list, the format that the guest
// produces with "plutil -convert xml1 -o -".
func ParsePlistXML(data []byte) (Plist, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return Plist{}, errors.New("the property list is empty")
			}
			return Plist{}, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "plist":
			val, err := parseValue(dec)
			if err != nil {
				return Plist{}, err
			}
			return Plist{Root: val}, nil
		case "dict", "array", "string", "integer", "real", "true", "false", "data", "date":
			val, err := parseValue(dec)
			if err != nil {
				return Plist{}, err
			}
			return Plist{Root: val}, nil
		default:
			return Plist{}, fmt.Errorf("unexpected property list element %q", start.Name.Local)
		}
	}
}

func parseValue(dec *xml.Decoder) (Value, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Only whitespace may sit between the elements of a property list.
			if strings.TrimSpace(string(t)) == "" {
				continue
			}
			return string(t), nil
		case xml.StartElement:
			return parseElement(dec, t)
		default:
			return nil, errors.New("unexpected property list token")
		}
	}
}

func parseElement(dec *xml.Decoder, start xml.StartElement) (Value, error) {
	switch start.Name.Local {
	case "dict":
		return parseDict(dec)
	case "array":
		return parseArray(dec)
	case "string":
		return readText(dec)
	case "integer":
		text, err := readText(dec)
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(strings.TrimSpace(text), 0, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q", text)
		}
		return n, nil
	case "real":
		text, err := readText(dec)
		if err != nil {
			return nil, err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid real %q", text)
		}
		return f, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "data":
		text, err := readText(dec)
		if err != nil {
			return nil, err
		}
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(text), ""))
		if err != nil {
			return nil, fmt.Errorf("invalid data: %w", err)
		}
		return raw, nil
	case "date":
		return readText(dec)
	default:
		return nil, fmt.Errorf("unexpected property list element %q", start.Name.Local)
	}
}

func parseDict(dec *xml.Decoder) (map[string]any, error) {
	out := map[string]any{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "key" {
				return nil, fmt.Errorf("unexpected dictionary element %q", t.Name.Local)
			}
			key, err := readText(dec)
			if err != nil {
				return nil, err
			}
			val, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			out[strings.TrimSpace(key)] = val
		case xml.EndElement:
			if t.Name.Local == "dict" {
				return out, nil
			}
		}
	}
}

func parseArray(dec *xml.Decoder) ([]any, error) {
	out := []any{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			val, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		case xml.EndElement:
			if t.Name.Local == "array" {
				return out, nil
			}
		}
	}
}

func readText(dec *xml.Decoder) (string, error) {
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.EndElement:
			return sb.String(), nil
		}
	}
}

// Strings returns every string of a value that is a string or an array of
// strings.
func Strings(v Value) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, Strings(item)...)
		}
		return out
	}
	return nil
}

// String returns the first string of a value.
func String(v Value) string {
	if s := Strings(v); len(s) > 0 {
		return s[0]
	}
	return ""
}
