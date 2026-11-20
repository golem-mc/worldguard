// Package yamlfile reads and writes YAML files the way WorldEdit's
// YAMLProcessor does: dotted paths, getters that write their default back
// for a missing key, and a header kept on save.
package yamlfile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// File is a YAML document of nested mappings.
type File struct {
	Path string
	// WriteDefaults makes a getter store its default for a missing key.
	WriteDefaults bool
	// Header is written before the document on save, each line prefixed by
	// nothing (it holds its own "#").
	Header string
	// Compact writes collections of plain values in flow style, as
	// WorldEdit's YAMLFormat.COMPACT does; otherwise everything is block
	// style.
	Compact bool

	root *yaml.Node
}

// New answers an empty file at path.
func New(path string, writeDefaults bool) *File {
	return &File{Path: path, WriteDefaults: writeDefaults, root: &yaml.Node{Kind: yaml.MappingNode}}
}

// Load reads the file at path; a missing file is empty, not an error.
func Load(path string, writeDefaults bool) (*File, error) {
	f := New(path, writeDefaults)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	return f, f.Parse(data)
}

// Parse replaces the document with data.
func (f *File) Parse(data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	f.root = &yaml.Node{Kind: yaml.MappingNode}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		f.root = doc.Content[0]
	}
	return nil
}

// find answers the node at a dotted path, nil when missing.
func (f *File) find(path string) *yaml.Node {
	n := f.root
	for p := range strings.SplitSeq(path, ".") {
		if n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == p {
				next = n.Content[i+1]
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// Get answers the value at path decoded into Go values (bool, int,
// float64, string, []any, map[string]any), false when missing or null.
func (f *File) Get(path string) (any, bool) {
	n := f.find(path)
	if n == nil || n.Tag == "!!null" {
		return nil, false
	}
	var v any
	if err := n.Decode(&v); err != nil || v == nil {
		return nil, false
	}
	return normalize(v), true
}

// normalize turns map[any]any into map[string]any throughout.
func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = normalize(e)
		}
		return x
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[fmt.Sprint(k)] = normalize(e)
		}
		return m
	case []any:
		for i, e := range x {
			x[i] = normalize(e)
		}
	}
	return v
}

// Has reports whether path holds a value.
func (f *File) Has(path string) bool {
	_, ok := f.Get(path)
	return ok
}

func (f *File) missing(path string, def any) {
	if f.WriteDefaults {
		f.Set(path, def)
	}
}

// Bool answers the boolean at path, def when missing or not a boolean.
func (f *File) Bool(path string, def bool) bool {
	v, ok := f.Get(path)
	if !ok {
		f.missing(path, def)
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// Int answers the number at path as an int, def when missing or not a
// number.
func (f *File) Int(path string, def int) int {
	v, ok := f.Get(path)
	if !ok {
		f.missing(path, def)
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case uint64:
		return int(n)
	}
	return def
}

// Double answers the number at path, def when missing or not a number.
func (f *File) Double(path string, def float64) float64 {
	v, ok := f.Get(path)
	if !ok {
		f.missing(path, def)
		return def
	}
	switch n := v.(type) {
	case int:
		return float64(n)
	case float64:
		return n
	}
	return def
}

// String answers the value at path as text, def when missing.
func (f *File) String(path, def string) string {
	v, ok := f.Get(path)
	if !ok {
		f.missing(path, def)
		return def
	}
	return Stringify(v)
}

// Stringify answers a value as Java's toString would.
func Stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return JavaDouble(x)
	}
	return fmt.Sprint(v)
}

// StringList answers the list at path with each element as text; a missing
// list is def, written back when def is not nil.
func (f *File) StringList(path string, def []string) []string {
	v, ok := f.Get(path)
	list, isList := v.([]any)
	if !ok || !isList {
		if !ok && def != nil {
			f.missing(path, def)
		}
		if def == nil {
			return []string{}
		}
		return def
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e != nil {
			out = append(out, Stringify(e))
		}
	}
	return out
}

// Keys answers the keys of the mapping at path in file order, nil when it
// is no mapping.
func (f *File) Keys(path string) []string {
	n := f.root
	if path != "" {
		n = f.find(path)
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

// Set stores v at path, creating the mappings on the way; v is a Go value
// as Get answers them, or a *yaml.Node.
func (f *File) Set(path string, v any) {
	parts := strings.Split(path, ".")
	n := f.root
	for i, p := range parts {
		var next *yaml.Node
		idx := -1
		for j := 0; j+1 < len(n.Content); j += 2 {
			if n.Content[j].Value == p {
				next, idx = n.Content[j+1], j+1
			}
		}
		if i == len(parts)-1 {
			val := f.encode(v)
			if idx >= 0 {
				n.Content[idx] = val
			} else {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: p}, val)
			}
			return
		}
		if next == nil || next.Kind != yaml.MappingNode {
			next = &yaml.Node{Kind: yaml.MappingNode}
			if idx >= 0 {
				n.Content[idx] = next
			} else {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: p}, next)
			}
		}
		n = next
	}
}

// Remove deletes the value at path.
func (f *File) Remove(path string) {
	parent, last := f.root, path
	if i := strings.LastIndex(path, "."); i >= 0 {
		parent, last = f.find(path[:i]), path[i+1:]
	}
	if parent == nil || parent.Kind != yaml.MappingNode {
		return
	}
	for j := 0; j+1 < len(parent.Content); j += 2 {
		if parent.Content[j].Value == last {
			parent.Content = slices.Delete(parent.Content, j, j+2)
			return
		}
	}
}

// Clear empties the document.
func (f *File) Clear() { f.root = &yaml.Node{Kind: yaml.MappingNode} }

// encode turns a Go value into a node: maps keep sorted key order unless
// given as an ordered *Map.
func (f *File) encode(v any) *yaml.Node {
	switch x := v.(type) {
	case *yaml.Node:
		return x
	case *Map:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for i, k := range x.Keys {
			n.Content = append(n.Content, scalar(k, "!!str"), f.encode(x.Values[i]))
		}
		f.style(n)
		return n
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			n.Content = append(n.Content, scalar(k, "!!str"), f.encode(x[k]))
		}
		f.style(n)
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range x {
			n.Content = append(n.Content, f.encode(e))
		}
		f.style(n)
		return n
	case []string:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range x {
			n.Content = append(n.Content, f.encode(e))
		}
		f.style(n)
		return n
	case nil:
		return scalar("null", "!!null")
	case bool:
		return scalar(strconv.FormatBool(x), "!!bool")
	case int:
		return scalar(strconv.Itoa(x), "!!int")
	case int64:
		return scalar(strconv.FormatInt(x, 10), "!!int")
	case float32:
		return scalar(JavaDouble(float64(x)), "!!float")
	case float64:
		return scalar(JavaDouble(x), "!!float")
	case string:
		return scalar(x, "!!str")
	}
	return scalar(fmt.Sprint(v), "!!str")
}

// style makes an empty collection, or with Compact one of plain values
// only, flow style.
func (f *File) style(n *yaml.Node) {
	if len(n.Content) == 0 {
		n.Style = yaml.FlowStyle
		return
	}
	if !f.Compact {
		return
	}
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return
		}
	}
	n.Style = yaml.FlowStyle
}

func scalar(v, tag string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: tag}
	// A string that would read back as another type is quoted, also for
	// SnakeYAML's YAML 1.1, where yes/no/on/off are booleans.
	if tag == "!!str" {
		var probe any
		switch err := yaml.Unmarshal([]byte(v), &probe); {
		case strings.ContainsAny(v, "\n"):
			n.Style = yaml.DoubleQuotedStyle
		case err != nil || probe != v || v == "" || yaml11Bools[v]:
			n.Style = yaml.SingleQuotedStyle
		}
	}
	return n
}

var yaml11Bools = map[string]bool{
	"yes": true, "Yes": true, "YES": true, "no": true, "No": true, "NO": true,
	"on": true, "On": true, "ON": true, "off": true, "Off": true, "OFF": true,
}

// Map is a mapping that keeps its keys in the order given.
type Map struct {
	Keys   []string
	Values []any
}

// Put appends a key.
func (m *Map) Put(k string, v any) {
	m.Keys = append(m.Keys, k)
	m.Values = append(m.Values, v)
}

// JavaDouble formats d as Java's Double.toString does for the values a
// config holds: "1.0", "0.5", "1.0E10".
func JavaDouble(d float64) string {
	if math.IsInf(d, 0) || math.IsNaN(d) {
		return strconv.FormatFloat(d, 'g', -1, 64)
	}
	abs := math.Abs(d)
	if abs != 0 && (abs < 1e-3 || abs >= 1e7) {
		s := strconv.FormatFloat(d, 'E', -1, 64)
		mant, exp, _ := strings.Cut(s, "E")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		e, _ := strconv.Atoi(exp)
		return mant + "E" + strconv.Itoa(e)
	}
	s := strconv.FormatFloat(d, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// Encode answers the document as YAML, header first.
func (f *File) Encode() ([]byte, error) {
	var buf bytes.Buffer
	if f.Header != "" {
		buf.WriteString(strings.ReplaceAll(f.Header, "\r\n", "\n"))
		if !strings.HasSuffix(f.Header, "\n") {
			buf.WriteByte('\n')
		}
	}
	if len(f.root.Content) == 0 {
		buf.WriteString("{}\n")
		return buf.Bytes(), nil
	}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(f.root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save writes the file, through a temporary file renamed over it.
func (f *File) Save() error {
	data, err := f.Encode()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return err
	}
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}
