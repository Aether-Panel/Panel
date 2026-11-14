package models

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"gorm.io/gorm"
)

type Template struct {
	skypanel.Server `gorm:"-"`

	Name     string `gorm:"column:name;size:100;primaryKey" json:"name"`
	RawValue string `gorm:"column:raw_value;not null;type:mediumtext" json:"-"`

	Readme string `gorm:"column:readme;size:4000" json:"readme,omitempty"`

	// written is the definition as its author wrote it (compact, without name and readme),
	// so the panel stores and serves it with the keys in their order and with fields
	// skypanel.Server does not know; nil when the template was built in Go
	written []byte
} // @name Template

func (t *Template) AfterFind(*gorm.DB) error {
	err := json.NewDecoder(strings.NewReader(t.RawValue)).Decode(&t.Server)
	if err != nil {
		return err
	}
	// a row that is not an object (never written by the panel) is served from Server
	t.written, _ = writtenDefinition([]byte(t.RawValue))
	t.RawValue = ""
	return nil
}

func (t *Template) BeforeSave(*gorm.DB) error {
	if written := t.asWritten(); written != nil {
		t.RawValue = string(written)
		return nil
	}
	data, err := json.Marshal(&t.Server)
	if err != nil {
		return err
	}
	t.RawValue = string(data)
	return nil
}

// UnmarshalJSON reads a template as before (name and readme included) and keeps the JSON as
// written: repository templates are served with their variables in the author's order
func (t *Template) UnmarshalJSON(data []byte) error {
	type plain Template // without these methods, so this does not call itself
	if err := json.Unmarshal(data, (*plain)(t)); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil // null leaves the template as it was
	}
	written, err := writtenDefinition(data)
	if err != nil {
		return err
	}
	t.written = written
	return nil
}

// MarshalJSON answers the definition as written followed by name and readme, the same shape
// as before (Server's fields, then name and readme) but without reordering the keys or
// dropping the ones skypanel.Server does not know
func (t Template) MarshalJSON() ([]byte, error) {
	written := t.asWritten()
	if written == nil {
		type plain Template
		return json.Marshal(plain(t))
	}

	var buf bytes.Buffer
	buf.Write(written[:len(written)-1]) // a compact object: everything but its closing brace
	if len(written) > len("{}") {
		buf.WriteByte(',')
	}
	if err := writeMember(&buf, "name", t.Name); err != nil {
		return nil, err
	}
	if t.Readme != "" {
		buf.WriteByte(',')
		if err := writeMember(&buf, "readme", t.Readme); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// SetDefinition makes data, a JSON object, the template's definition: Server is decoded from
// it and it is stored and served as written. Its name and readme keys are left out, the
// template's columns hold them.
func (t *Template) SetDefinition(data []byte) error {
	written, err := writtenDefinition(data)
	if err != nil {
		return err
	}
	var server skypanel.Server
	if err := json.Unmarshal(written, &server); err != nil {
		return err
	}
	t.Server = server
	t.written = written
	return nil
}

// SetField sets one top-level key of the definition, in its place when the key is there and
// at the end when it is not, and decodes Server again
func (t *Template) SetField(key string, value any) error {
	written := t.asWritten()
	if written == nil {
		data, err := json.Marshal(&t.Server)
		if err != nil {
			return err
		}
		written = data
	}
	members, err := readObject(written)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	replaced := false
	kept := members[:0]
	for _, m := range members {
		if m.key == key {
			if replaced {
				continue // a repeated key: JSON readers take the last one, which is this value now
			}
			m.value, replaced = encoded, true
		}
		kept = append(kept, m)
	}
	if !replaced {
		kept = append(kept, jsonMember{key: key, value: encoded})
	}

	data, err := writeObject(kept)
	if err != nil {
		return err
	}
	return t.SetDefinition(data)
}

// asWritten is the definition as written, or nil when there is none or Server has been
// changed since it was read (a migration, for instance): then Server is what counts
func (t *Template) asWritten() []byte {
	if t.written == nil {
		return nil
	}
	var server skypanel.Server
	if json.Unmarshal(t.written, &server) != nil || !reflect.DeepEqual(server, t.Server) {
		return nil
	}
	return t.written
}

// errNotAnObject is what a template definition that is not a JSON object gets
var errNotAnObject = skypanel.CreateError("a template must be a JSON object", "ErrInvalidTemplate")

// jsonMember is one key of a JSON object and its value as written
type jsonMember struct {
	key   string
	value json.RawMessage
}

// writtenDefinition is the object in data, compact and without name and readme, with its
// keys in the order they were written
func writtenDefinition(data []byte) ([]byte, error) {
	members, err := readObject(data)
	if err != nil {
		return nil, err
	}
	definition := members[:0]
	for _, m := range members {
		if m.key != "name" && m.key != "readme" {
			definition = append(definition, m)
		}
	}
	return writeObject(definition)
}

// readObject lists the keys of the JSON object at the start of data in their order, the way
// encoding/json reads a request body (anything after the object is not read)
func readObject(data []byte) ([]jsonMember, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil {
		return nil, err
	} else if tok != json.Delim('{') {
		return nil, errNotAnObject
	}

	members := []jsonMember{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string) // inside an object the decoder only hands out string keys
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{key: key, value: value})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, err
	}
	return members, nil
}

// writeObject writes members as one compact JSON object
func writeObject(members []jsonMember) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := writeMember(&buf, m.key, m.value); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// writeMember writes "key":value, compact
func writeMember(buf *bytes.Buffer, key string, value any) error {
	encodedKey, err := json.Marshal(key)
	if err != nil {
		return err
	}
	buf.Write(encodedKey)
	buf.WriteByte(':')

	raw, ok := value.(json.RawMessage)
	if !ok {
		if raw, err = json.Marshal(value); err != nil {
			return err
		}
	}
	return json.Compact(buf, raw)
}
