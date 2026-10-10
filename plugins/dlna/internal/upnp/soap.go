// Package upnp implements the parts of UPnP Device Architecture 1.0 that
// DLNA devices speak over HTTP: SOAP actions and faults, device
// descriptions, and GENA eventing.
package upnp

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Arg is a named argument or state variable.
type Arg struct{ Name, Value string }

// Action is a SOAP action request.
type Action struct {
	ServiceType, Name string
	Args              map[string]string
}

// maxBody bounds the SOAP messages read.
const maxBody = 1 << 20

// ReadAction reads the action a control request invokes. The action is
// named by the SOAPACTION header, "serviceType#name" in quotes, and by the
// body's first element, which carries the arguments.
func ReadAction(r *http.Request) (Action, error) {
	h := strings.Trim(r.Header.Get("Soapaction"), `" `)
	serviceType, name, ok := strings.Cut(h, "#")
	if !ok {
		return Action{}, fmt.Errorf("SOAPACTION header %q", h)
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return Action{}, err
	}
	elem, args, err := parseBody(body)
	if err != nil {
		return Action{}, err
	}
	if elem != name {
		return Action{}, fmt.Errorf("body invokes %s, SOAPACTION %s", elem, name)
	}
	return Action{ServiceType: serviceType, Name: name, Args: args}, nil
}

// parseBody returns the first element of a SOAP envelope's body and the
// text of its children by local name.
func parseBody(body []byte) (string, map[string]string, error) {
	d := xml.NewDecoder(bytes.NewReader(body))
	inBody := false
	for {
		tok, err := d.Token()
		if err != nil {
			return "", nil, fmt.Errorf("SOAP envelope: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !inBody {
			inBody = start.Name.Local == "Body"
			continue
		}
		args, err := children(d)
		return start.Name.Local, args, err
	}
}

// children reads the elements within the current one, up to its end, as
// text by local name; nested markup is kept as text.
func children(d *xml.Decoder) (map[string]string, error) {
	args := map[string]string{}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var v struct {
				Text string `xml:",innerxml"`
			}
			if err := d.DecodeElement(&v, &t); err != nil {
				return nil, err
			}
			args[t.Name.Local] = unescapeInner(v.Text)
		case xml.EndElement:
			return args, nil
		}
	}
}

// unescapeInner turns an element's inner XML into its text, leaving
// nested markup as it is.
func unescapeInner(s string) string {
	if !strings.ContainsAny(s, "&<") {
		return s
	}
	var b strings.Builder
	d := xml.NewDecoder(strings.NewReader("<x>" + s + "</x>"))
	depth := 0
	for {
		tok, err := d.RawToken()
		if err != nil {
			return b.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			if depth > 0 {
				b.Write(t)
			}
		case xml.StartElement:
			depth++
			if depth > 1 {
				b.WriteString(rawStart(t))
			}
		case xml.EndElement:
			if depth > 1 {
				b.WriteString("</" + qname(t.Name) + ">")
			}
			depth--
		}
	}
}

func qname(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

func rawStart(t xml.StartElement) string {
	var b strings.Builder
	b.WriteString("<" + qname(t.Name))
	for _, a := range t.Attr {
		b.WriteString(" " + qname(a.Name) + `="`)
		_ = xml.EscapeText(&b, []byte(a.Value))
		b.WriteString(`"`)
	}
	b.WriteString(">")
	return b.String()
}

const (
	envelopeStart = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>`
	envelopeEnd = `</s:Body></s:Envelope>`
)

// Envelope encodes an action's request or response: the element named
// name in the service's namespace, holding the arguments in order.
func Envelope(serviceType, name string, args []Arg) []byte {
	var b bytes.Buffer
	b.WriteString(envelopeStart)
	fmt.Fprintf(&b, `<u:%s xmlns:u="%s">`, name, escape(serviceType))
	for _, a := range args {
		fmt.Fprintf(&b, "<%s>%s</%s>", a.Name, escape(a.Value), a.Name)
	}
	fmt.Fprintf(&b, "</u:%s>", name)
	b.WriteString(envelopeEnd)
	return b.Bytes()
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// WriteResponse answers an action with its out arguments.
func WriteResponse(w http.ResponseWriter, a Action, args []Arg) {
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.Header().Set("Ext", "")
	_, _ = w.Write(Envelope(a.ServiceType, a.Name+"Response", args))
}

// Fault is a UPnP error.
type Fault struct {
	Code        int
	Description string
}

func (f *Fault) Error() string { return fmt.Sprintf("UPnP error %d: %s", f.Code, f.Description) }

// UPnP errors of the device architecture and of the AV services.
var (
	ErrInvalidAction   = &Fault{401, "Invalid Action"}
	ErrInvalidArgs     = &Fault{402, "Invalid Args"}
	ErrActionFailed    = &Fault{501, "Action Failed"}
	ErrNoSuchObject    = &Fault{701, "No such object"}
	ErrBadSearch       = &Fault{708, "Unsupported or invalid search criteria"}
	ErrBadSort         = &Fault{709, "Unsupported or invalid sort criteria"}
	ErrNoSuchContainer = &Fault{710, "No such container"}
	ErrCannotProcess   = &Fault{720, "Cannot process the request"}
)

// WriteFault answers an action with an error.
func WriteFault(w http.ResponseWriter, f *Fault) {
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	w.Header().Set("Ext", "")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `%s<s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail>`+
		`<UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>%d</errorCode><errorDescription>%s</errorDescription></UPnPError>`+
		`</detail></s:Fault>%s`, envelopeStart, f.Code, escape(f.Description), envelopeEnd)
}

// Call invokes an action on a device's control URL and returns its out
// arguments; a UPnP error is a *Fault.
func Call(ctx context.Context, c *http.Client, controlURL, serviceType, action string, args []Arg) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, bytes.NewReader(Envelope(serviceType, action, args)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("Soapaction", `"`+serviceType+"#"+action+`"`)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	elem, out, err := parseBody(body)
	switch {
	case err != nil && resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", action, resp.Status)
	case err != nil:
		return nil, fmt.Errorf("%s: %w", action, err)
	case elem == "Fault":
		return nil, parseFault(out["detail"])
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", action, resp.Status)
	}
	return out, nil
}

// parseFault reads the UPnPError within a fault's detail.
func parseFault(detail string) error {
	var e struct {
		Code        string `xml:"errorCode"`
		Description string `xml:"errorDescription"`
	}
	if err := xml.Unmarshal([]byte(detail), &e); err != nil {
		return errors.New("SOAP fault without a UPnP error")
	}
	code, _ := strconv.Atoi(strings.TrimSpace(e.Code))
	return &Fault{Code: code, Description: strings.TrimSpace(e.Description)}
}
