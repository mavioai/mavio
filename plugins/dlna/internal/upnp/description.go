package upnp

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Description is a device description, as read from a device.
type Description struct {
	// URLBase, when the device gives one, resolves its relative URLs;
	// otherwise the description's own URL does.
	URLBase string `xml:"URLBase"`
	Device  Device `xml:"device"`
}

// Device is a device of a description.
type Device struct {
	DeviceType   string    `xml:"deviceType"`
	FriendlyName string    `xml:"friendlyName"`
	Manufacturer string    `xml:"manufacturer"`
	ModelName    string    `xml:"modelName"`
	ModelNumber  string    `xml:"modelNumber"`
	UDN          string    `xml:"UDN"`
	Services     []Service `xml:"serviceList>service"`
	Devices      []Device  `xml:"deviceList>device"`
}

// Service is a service of a device.
type Service struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	SCPDURL     string `xml:"SCPDURL"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

// Find returns the first device of a type among the device and those it
// embeds.
func (d *Device) Find(deviceType string) (*Device, bool) {
	if sameType(d.DeviceType, deviceType) {
		return d, true
	}
	for i := range d.Devices {
		if found, ok := d.Devices[i].Find(deviceType); ok {
			return found, true
		}
	}
	return nil, false
}

// Service returns the device's service of a type.
func (d *Device) Service(serviceType string) (*Service, bool) {
	for i := range d.Services {
		if sameType(d.Services[i].ServiceType, serviceType) {
			return &d.Services[i], true
		}
	}
	return nil, false
}

// sameType compares UPnP types without their versions: a device of a later
// version serves the earlier ones.
func sameType(a, b string) bool {
	trim := func(s string) string {
		if i := strings.LastIndexByte(s, ':'); i > 0 && strings.HasPrefix(s, "urn:") {
			return s[:i]
		}
		return s
	}
	return trim(a) == trim(b)
}

// FetchDescription reads a device's description and resolves its service
// URLs to absolute ones.
func FetchDescription(ctx context.Context, c *http.Client, location string) (*Description, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("description %s: %s", location, resp.Status)
	}
	var d Description
	if err := xml.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&d); err != nil {
		return nil, fmt.Errorf("description %s: %w", location, err)
	}
	base, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	if d.URLBase != "" {
		if u, err := base.Parse(strings.TrimSpace(d.URLBase)); err == nil {
			base = u
		}
	}
	resolveURLs(&d.Device, base)
	return &d, nil
}

func resolveURLs(d *Device, base *url.URL) {
	resolve := func(s *string) {
		if u, err := base.Parse(strings.TrimSpace(*s)); err == nil && *s != "" {
			*s = u.String()
		}
	}
	for i := range d.Services {
		s := &d.Services[i]
		resolve(&s.SCPDURL)
		resolve(&s.ControlURL)
		resolve(&s.EventSubURL)
	}
	for i := range d.Devices {
		resolveURLs(&d.Devices[i], base)
	}
}
