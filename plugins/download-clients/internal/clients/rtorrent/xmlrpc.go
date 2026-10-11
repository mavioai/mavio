package rtorrent

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// encodeCall writes an XML-RPC method call as Radarr's
// XmlRpcRequestBuilder: strings, ints, byte slices as base64, string
// slices as arrays and string maps as structs.
func encodeCall(method string, params ...any) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?><methodCall><methodName>`)
	if err := xml.EscapeText(&b, []byte(method)); err != nil {
		return nil, err
	}
	b.WriteString("</methodName>")
	if len(params) > 0 {
		b.WriteString("<params>")
		for _, p := range params {
			b.WriteString("<param>")
			if err := encodeValue(&b, p); err != nil {
				return nil, err
			}
			b.WriteString("</param>")
		}
		b.WriteString("</params>")
	}
	b.WriteString("</methodCall>")
	return b.Bytes(), nil
}

func encodeValue(b *bytes.Buffer, v any) error {
	text := func(tag, s string) error {
		fmt.Fprintf(b, "<value><%s>", tag)
		if err := xml.EscapeText(b, []byte(s)); err != nil {
			return err
		}
		fmt.Fprintf(b, "</%s></value>", tag)
		return nil
	}
	switch v := v.(type) {
	case string:
		return text("string", v)
	case int:
		return text("int", strconv.Itoa(v))
	case []byte:
		return text("base64", base64.StdEncoding.EncodeToString(v))
	case []string:
		b.WriteString("<value><array><data>")
		for _, s := range v {
			if err := text("string", s); err != nil {
				return err
			}
		}
		b.WriteString("</data></array></value>")
	case map[string]string:
		b.WriteString("<value><struct>")
		for _, k := range slices.Sorted(maps.Keys(v)) {
			s := v[k]
			b.WriteString("<member><name>")
			if err := xml.EscapeText(b, []byte(k)); err != nil {
				return err
			}
			b.WriteString("</name>")
			if err := text("string", s); err != nil {
				return err
			}
			b.WriteString("</member>")
		}
		b.WriteString("</struct></value>")
	default:
		return fmt.Errorf("xml-rpc: unhandled argument type %T", v)
	}
	return nil
}

// value is an XML-RPC value.
type value struct {
	String  *string  `xml:"string"`
	Int     *string  `xml:"int"`
	I4      *string  `xml:"i4"`
	I8      *string  `xml:"i8"`
	Boolean *string  `xml:"boolean"`
	Double  *string  `xml:"double"`
	Base64  *string  `xml:"base64"`
	Array   *array   `xml:"array"`
	Struct  *members `xml:"struct"`
	// Text is an untyped value, a string.
	Text string `xml:",chardata"`
}

type array struct {
	Values []value `xml:"data>value"`
}

type members struct {
	Members []member `xml:"member"`
}

type member struct {
	Name  string `xml:"name"`
	Value value  `xml:"value"`
}

// str returns a value as a string; numbers are given as written.
func (v *value) str() string {
	for _, s := range []*string{v.String, v.I8, v.I4, v.Int, v.Double, v.Boolean, v.Base64} {
		if s != nil {
			return *s
		}
	}
	if v.Array != nil || v.Struct != nil {
		return ""
	}
	return v.Text
}

// int returns a value as an integer; zero when it is not one.
func (v *value) int() int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v.str()), 10, 64)
	return n
}

// values returns the elements of an array value.
func (v *value) values() []value {
	if v.Array == nil {
		return nil
	}
	return v.Array.Values
}

// member returns the member of a struct value by name.
func (v *value) member(name string) *value {
	if v.Struct == nil {
		return nil
	}
	for i := range v.Struct.Members {
		if v.Struct.Members[i].Name == name {
			return &v.Struct.Members[i].Value
		}
	}
	return nil
}

// methodResponse is an XML-RPC response: a value or a fault.
type methodResponse struct {
	Params []value `xml:"params>param>value"`
	Fault  *value  `xml:"fault>value"`
}

// faultError is an XML-RPC fault.
type faultError struct {
	Code    int64
	Message string
}

func (e *faultError) Error() string {
	return fmt.Sprintf("rTorrent returned error code %d: %s", e.Code, e.Message)
}

// decodeResponse reads an XML-RPC response and returns its value, or its
// fault as a *faultError.
func decodeResponse(body []byte) (*value, error) {
	var r methodResponse
	if err := xml.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("read rTorrent response: %w", err)
	}
	if r.Fault != nil {
		f := &faultError{}
		if c := r.Fault.member("faultCode"); c != nil {
			f.Code = c.int()
		}
		if s := r.Fault.member("faultString"); s != nil {
			f.Message = s.str()
		}
		return nil, f
	}
	if len(r.Params) == 0 {
		return &value{}, nil
	}
	return &r.Params[0], nil
}
