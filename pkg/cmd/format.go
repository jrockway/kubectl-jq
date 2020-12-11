package cmd

import (
	"bytes"
	"encoding/json"

	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"gopkg.in/yaml.v3"
)

const outputHelp = "Output format. One of: json|jsoncompact|jsonpretty|yaml|yamlnosep"

type Formatter interface {
	Marshal(interface{}) ([]byte, error)
}

type compactJSON struct{}

func (compactJSON) Marshal(x interface{}) ([]byte, error) {
	return json.Marshal(x)
}

type indentedJSON struct{}

func (indentedJSON) Marshal(x interface{}) ([]byte, error) {
	return json.MarshalIndent(x, "", "    ")
}

type yamlMarshaler struct {
	prefix []byte
}

func (m *yamlMarshaler) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	bs := bytes.TrimRight(buf.Bytes(), "\n")
	if prefix := m.prefix; prefix != nil {
		return bytes.Join([][]byte{prefix, bs}, nil), nil
	}
	return bs, nil
}

func outputFormat(format string) Formatter {
	switch format {
	case "jsonpretty":
		f := prettyjson.NewFormatter()
		f.Indent = 4
		f.StringColor = color.New(color.FgGreen)
		f.BoolColor = color.New(color.FgYellow)
		f.NumberColor = color.New(color.FgCyan)
		f.NullColor = color.New(color.FgHiBlack)
		return f
	case "jsoncompact":
		return compactJSON{}
	case "json":
		return indentedJSON{}
	case "yaml":
		return &yamlMarshaler{prefix: []byte("---\n")}
	case "yamlnosep":
		return &yamlMarshaler{prefix: nil}
	default:
		return indentedJSON{}
	}
}
