// MacXVM - OpenCore Plist (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Value interface {
	write(enc *xml.Encoder) error
}

type String string

type Integer int64

type Real float64

type Boolean bool

type Data []byte

type Array struct {
	values []Value
}

func NewArray(values ...Value) *Array {
	return &Array{values: values}
}

func (a *Array) Append(v Value) { a.values = append(a.values, v) }

func (a *Array) Len() int { return len(a.values) }

func (a *Array) At(i int) (Value, bool) {
	if i < 0 || i >= len(a.values) {
		return nil, false
	}
	return a.values[i], true
}

type Dict struct {
	keys   []string
	values map[string]Value
}

func NewDict() *Dict {
	return &Dict{values: map[string]Value{}}
}

func (d *Dict) Set(key string, value Value) {
	if d.values == nil {
		d.values = map[string]Value{}
	}
	if _, ok := d.values[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.values[key] = value
}

func (d *Dict) Get(key string) (Value, bool) {
	if d == nil || d.values == nil {
		return nil, false
	}
	v, ok := d.values[key]
	return v, ok
}

func (d *Dict) Keys() []string {
	if d == nil {
		return nil
	}
	out := append([]string(nil), d.keys...)
	return out
}

func (d *Dict) Remove(key string) {
	if d == nil || d.values == nil {
		return
	}
	if _, ok := d.values[key]; !ok {
		return
	}
	delete(d.values, key)
	for i, existing := range d.keys {
		if existing == key {
			d.keys = append(d.keys[:i], d.keys[i+1:]...)
			break
		}
	}
}

func (d *Dict) Path(path []string) (Value, bool) {
	if len(path) == 0 {
		return d, true
	}
	current, ok := d.Get(path[0])
	if !ok {
		return nil, false
	}
	for _, key := range path[1:] {
		next, ok := current.(*Dict)
		if !ok {
			return nil, false
		}
		current, ok = next.Get(key)
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func (d *Dict) SetPath(path []string, value Value) error {
	if len(path) == 0 {
		return fmt.Errorf("the path of a plist key cannot be empty")
	}
	current := d
	for _, key := range path[:len(path)-1] {
		next, ok := current.Get(key)
		if !ok {
			child := NewDict()
			current.Set(key, child)
			current = child
			continue
		}
		child, ok := next.(*Dict)
		if !ok {
			return fmt.Errorf("the plist key %q holds a value, not a dictionary", key)
		}
		current = child
	}
	current.Set(path[len(path)-1], value)
	return nil
}

func (d *Dict) String(path []string) (string, bool) {
	value, ok := d.Path(path)
	if !ok {
		return "", false
	}
	s, ok := value.(String)
	return string(s), ok
}

func (d *Dict) Bool(path []string) (bool, bool) {
	value, ok := d.Path(path)
	if !ok {
		return false, false
	}
	b, ok := value.(Boolean)
	return bool(b), ok
}

func (d *Dict) SetString(path []string, value string) error {
	return d.SetPath(path, String(value))
}

func (d *Dict) SetBool(path []string, value bool) error {
	return d.SetPath(path, Boolean(value))
}

func (d *Dict) SetArray(path []string, value *Array) error {
	return d.SetPath(path, value)
}

func Parse(data []byte) (Value, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	var root Value
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the plist is not valid XML: %w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("the plist must not contain a DTD or another directive")
		case xml.ProcInst:
			continue
		case xml.StartElement:
			if t.Name.Local != "plist" {
				return nil, fmt.Errorf("the root element of a plist must be <plist>, not <%s>", t.Name.Local)
			}
			value, err := parseValue(decoder)
			if err != nil {
				return nil, err
			}
			root = value
			if err := skipRest(decoder); err != nil {
				return nil, err
			}
			return root, nil
		}
	}
	return nil, fmt.Errorf("the plist has no <plist> element")
}

func ParseDict(data []byte) (*Dict, error) {
	value, err := Parse(data)
	if err != nil {
		return nil, err
	}
	dict, ok := value.(*Dict)
	if !ok {
		return nil, fmt.Errorf("the plist does not hold a dictionary at its root")
	}
	return dict, nil
}

func skipRest(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("the plist is not valid XML: %w", err)
		}
		switch token.(type) {
		case xml.Directive:
			return fmt.Errorf("the plist must not contain a DTD or another directive")
		case xml.StartElement:
			if err := decoder.Skip(); err != nil {
				return fmt.Errorf("the plist is not valid XML: %w", err)
			}
		}
	}
}

func parseValue(decoder *xml.Decoder) (Value, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("the plist is not valid XML: %w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("the plist must not contain a DTD or another directive")
		case xml.StartElement:
			return parseElement(decoder, t)
		}
	}
}

func parseElement(decoder *xml.Decoder, start xml.StartElement) (Value, error) {
	switch start.Name.Local {
	case "dict":
		return parseDict(decoder)
	case "array":
		return parseArray(decoder)
	case "string":
		var s string
		if err := decoder.DecodeElement(&s, &start); err != nil {
			return nil, fmt.Errorf("the plist holds an invalid string: %w", err)
		}
		return String(s), nil
	case "integer":
		var s string
		if err := decoder.DecodeElement(&s, &start); err != nil {
			return nil, fmt.Errorf("the plist holds an invalid integer: %w", err)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the plist holds an invalid integer %q", s)
		}
		return Integer(n), nil
	case "real":
		var s string
		if err := decoder.DecodeElement(&s, &start); err != nil {
			return nil, fmt.Errorf("the plist holds an invalid real: %w", err)
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, fmt.Errorf("the plist holds an invalid real %q", s)
		}
		return Real(f), nil
	case "true":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return Boolean(true), nil
	case "false":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return Boolean(false), nil
	case "data":
		var s string
		if err := decoder.DecodeElement(&s, &start); err != nil {
			return nil, fmt.Errorf("the plist holds invalid data: %w", err)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(s), ""))
		if err != nil {
			return nil, fmt.Errorf("the plist holds invalid base64 data: %w", err)
		}
		return Data(raw), nil
	case "date":
		var s string
		if err := decoder.DecodeElement(&s, &start); err != nil {
			return nil, fmt.Errorf("the plist holds an invalid date: %w", err)
		}
		return String(strings.TrimSpace(s)), nil
	default:
		return nil, fmt.Errorf("the plist holds an unsupported element <%s>", start.Name.Local)
	}
}

func parseDict(decoder *xml.Decoder) (*Dict, error) {
	dict := NewDict()
	var pending string
	havePending := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("the plist holds an invalid dictionary: %w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("the plist must not contain a DTD or another directive")
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				var key string
				if err := decoder.DecodeElement(&key, &t); err != nil {
					return nil, fmt.Errorf("the plist holds an invalid key: %w", err)
				}
				pending = key
				havePending = true
			default:
				value, err := parseElement(decoder, t)
				if err != nil {
					return nil, err
				}
				if !havePending {
					return nil, fmt.Errorf("the plist holds a value without a key")
				}
				dict.Set(pending, value)
				havePending = false
			}
		case xml.EndElement:
			if t.Name.Local == "dict" {
				if havePending {
					return nil, fmt.Errorf("the plist ends with a key without a value")
				}
				return dict, nil
			}
		}
	}
}

func parseArray(decoder *xml.Decoder) (*Array, error) {
	array := NewArray()
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("the plist holds an invalid array: %w", err)
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("the plist must not contain a DTD or another directive")
		case xml.StartElement:
			value, err := parseElement(decoder, t)
			if err != nil {
				return nil, err
			}
			array.Append(value)
		case xml.EndElement:
			if t.Name.Local == "array" {
				return array, nil
			}
		}
	}
}

func Marshal(value Value) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "\t")
	if err := encoder.EncodeToken(xml.ProcInst{Target: "xml", Inst: []byte(`version="1.0" encoding="UTF-8"`)}); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "plist"}, Attr: []xml.Attr{{Name: xml.Name{Local: "version"}, Value: "1.0"}}}); err != nil {
		return nil, err
	}
	if err := value.write(encoder); err != nil {
		return nil, err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "plist"}}); err != nil {
		return nil, err
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}

func (s String) write(enc *xml.Encoder) error {
	return enc.EncodeElement(string(s), xml.StartElement{Name: xml.Name{Local: "string"}})
}

func (i Integer) write(enc *xml.Encoder) error {
	return enc.EncodeElement(strconv.FormatInt(int64(i), 10), xml.StartElement{Name: xml.Name{Local: "integer"}})
}

func (r Real) write(enc *xml.Encoder) error {
	return enc.EncodeElement(strconv.FormatFloat(float64(r), 'g', -1, 64), xml.StartElement{Name: xml.Name{Local: "real"}})
}

func (b Boolean) write(enc *xml.Encoder) error {
	name := "false"
	if bool(b) {
		name = "true"
	}
	start := xml.StartElement{Name: xml.Name{Local: name}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}})
}

func (d Data) write(enc *xml.Encoder) error {
	encoded := base64.StdEncoding.EncodeToString(d)
	return enc.EncodeElement(encoded, xml.StartElement{Name: xml.Name{Local: "data"}})
}

func (a *Array) write(enc *xml.Encoder) error {
	start := xml.StartElement{Name: xml.Name{Local: "array"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for _, value := range a.values {
		if err := value.write(enc); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "array"}})
}

func (d *Dict) write(enc *xml.Encoder) error {
	start := xml.StartElement{Name: xml.Name{Local: "dict"}}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for _, key := range d.keys {
		if err := enc.EncodeElement(key, xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
			return err
		}
		value, ok := d.Get(key)
		if !ok {
			continue
		}
		if err := value.write(enc); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}})
}
