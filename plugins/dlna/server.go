package main

import (
	"bytes"
	"embed"
	"encoding/xml"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
	"github.com/mavioai/mavio/plugins/dlna/internal/upnp"
)

// The media server's device and service types.
const (
	deviceType        = "urn:schemas-upnp-org:device:MediaServer:1"
	contentDirectory  = "urn:schemas-upnp-org:service:ContentDirectory:1"
	connectionManager = "urn:schemas-upnp-org:service:ConnectionManager:1"
	receiverRegistrar = "urn:microsoft.com:service:X_MS_MediaReceiverRegistrar:1"
)

//go:embed scpd
var scpds embed.FS

// services maps the short names of the media server's services in its
// URLs to their types and SCPD files.
var services = map[string]struct{ typ, id, scpd string }{
	"cds": {contentDirectory, "urn:upnp-org:serviceId:ContentDirectory", "scpd/ContentDirectory.xml"},
	"cms": {connectionManager, "urn:upnp-org:serviceId:ConnectionManager", "scpd/ConnectionManager.xml"},
	"mrr": {receiverRegistrar, "urn:microsoft.com:serviceId:X_MS_MediaReceiverRegistrar", "scpd/MediaReceiverRegistrar.xml"},
}

// routes returns the plugin's HTTP routes.
func (p *plugin) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /description.xml", p.mediaServerOnly(p.serveDescription))
	mux.HandleFunc("GET /icons/{file}", serveIcon)
	mux.HandleFunc("GET /scpd/{service}", p.mediaServerOnly(serveSCPD))
	mux.HandleFunc("POST /control/{service}", p.mediaServerOnly(p.serveControl))
	for _, method := range []string{"SUBSCRIBE", "UNSUBSCRIBE"} {
		mux.HandleFunc(method+" /events/{service}", p.mediaServerOnly(p.serveEvents))
	}
	mux.HandleFunc("GET /media/{item}/{file}", p.serveMedia)
	mux.HandleFunc("GET /subtitles/{item}/{file}", p.serveSubtitles)
	mux.HandleFunc("GET /images/{image}", p.serveImage)
	mux.HandleFunc("GET /play/{playback}/{file}", p.servePlayTo)
	return mux
}

// mediaServerOnly serves a route of the media server while it is on.
func (p *plugin) mediaServerOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _, _, ready := p.state()
		if !ready || !c.mediaServer() {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

// requestRoot returns the URL of the plugin's routes as the request
// reached them, through the server's proxy.
func (p *plugin) requestRoot(r *http.Request) string {
	_, _, info, _ := p.state()
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return p.root(host, info)
}

var descriptionTemplate = template.Must(template.New("description").Funcs(template.FuncMap{"x": xmlText}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0" xmlns:dlna="urn:schemas-dlna-org:device-1-0" xmlns:sec="http://www.sec.co.kr/dlna">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <dlna:X_DLNADOC>DMS-1.50</dlna:X_DLNADOC>
    <dlna:X_DLNADOC>M-DMS-1.50</dlna:X_DLNADOC>
    <sec:ProductCap>smi,DCM10,getMediaInfo.sec,getCaptionInfo.sec</sec:ProductCap>
    <sec:X_ProductCap>smi,DCM10,getMediaInfo.sec,getCaptionInfo.sec</sec:X_ProductCap>
    <deviceType>{{x .DeviceType}}</deviceType>
    <friendlyName>{{x .Name}}</friendlyName>
    <manufacturer>Mavio</manufacturer>
    <manufacturerURL>https://github.com/mavioai/mavio</manufacturerURL>
    <modelDescription>Mavio media server</modelDescription>
    <modelName>Mavio</modelName>
    <modelNumber>{{x .Version}}</modelNumber>
    <modelURL>https://github.com/mavioai/mavio</modelURL>
    <serialNumber>{{x .Serial}}</serialNumber>
    <UDN>{{x .UDN}}</UDN>
    <iconList>{{range .Icons}}
      <icon><mimetype>{{x .Type}}</mimetype><width>{{.Size}}</width><height>{{.Size}}</height><depth>24</depth><url>{{x $.Root}}/icons/{{x .File}}</url></icon>{{end}}
    </iconList>
    <serviceList>{{range .Services}}
      <service>
        <serviceType>{{x .Type}}</serviceType>
        <serviceId>{{x .ID}}</serviceId>
        <SCPDURL>{{x $.Root}}/scpd/{{x .Name}}</SCPDURL>
        <controlURL>{{x $.Root}}/control/{{x .Name}}</controlURL>
        <eventSubURL>{{x $.Root}}/events/{{x .Name}}</eventSubURL>
      </service>{{end}}
    </serviceList>
  </device>
</root>
`))

func (p *plugin) serveDescription(w http.ResponseWriter, r *http.Request) {
	c, _, info, _ := p.state()
	type svc struct{ Name, Type, ID string }
	data := struct {
		DeviceType, Name, Version, Serial, UDN, Root string
		Icons                                        []icon
		Services                                     []svc
	}{
		DeviceType: deviceType, Name: p.friendlyName(c, info), Version: p.version,
		Serial: strings.TrimPrefix(p.udn, "uuid:"), UDN: p.udn, Root: p.requestRoot(r), Icons: icons,
	}
	for _, name := range []string{"cds", "cms", "mrr"} {
		data.Services = append(data.Services, svc{name, services[name].typ, services[name].id})
	}
	var b bytes.Buffer
	if err := descriptionTemplate.Execute(&b, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write(b.Bytes())
}

func serveSCPD(w http.ResponseWriter, r *http.Request) {
	s, ok := services[r.PathValue("service")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := scpds.ReadFile(s.scpd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write(b)
}

func (p *plugin) serveEvents(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	switch name {
	case "cds":
		p.events.Handler(name, func() []upnp.Arg {
			return []upnp.Arg{{Name: "SystemUpdateID", Value: strconv.FormatUint(uint64(p.updateID.Load()), 10)}}
		})(w, r)
	case "cms":
		p.events.Handler(name, func() []upnp.Arg {
			return []upnp.Arg{{Name: "SourceProtocolInfo", Value: p.sourceProtocolInfo()}, {Name: "SinkProtocolInfo"}, {Name: "CurrentConnectionIDs", Value: "0"}}
		})(w, r)
	case "mrr":
		p.events.Handler(name, func() []upnp.Arg {
			return []upnp.Arg{
				{Name: "AuthorizationGrantedUpdateID", Value: "0"},
				{Name: "AuthorizationDeniedUpdateID", Value: "0"},
				{Name: "ValidationSucceededUpdateID", Value: "0"},
				{Name: "ValidationRevokedUpdateID", Value: "0"},
			}
		})(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveControl answers the actions of the media server's services.
func (p *plugin) serveControl(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	if _, ok := services[name]; !ok {
		http.NotFound(w, r)
		return
	}
	a, err := upnp.ReadAction(r)
	if err != nil {
		p.log.DebugContext(r.Context(), "bad SOAP request", "err", err)
		upnp.WriteFault(w, upnp.ErrInvalidAction)
		return
	}
	var out []upnp.Arg
	var fault *upnp.Fault
	switch name {
	case "cds":
		out, fault = p.contentDirectoryAction(r, a)
	case "cms":
		out, fault = p.connectionManagerAction(a)
	case "mrr":
		out, fault = receiverRegistrarAction(a)
	}
	if fault != nil {
		upnp.WriteFault(w, fault)
		return
	}
	upnp.WriteResponse(w, a, out)
}

func (p *plugin) connectionManagerAction(a upnp.Action) ([]upnp.Arg, *upnp.Fault) {
	switch a.Name {
	case "GetProtocolInfo":
		return []upnp.Arg{{Name: "Source", Value: p.sourceProtocolInfo()}, {Name: "Sink"}}, nil
	case "GetCurrentConnectionIDs":
		return []upnp.Arg{{Name: "ConnectionIDs", Value: "0"}}, nil
	case "GetCurrentConnectionInfo":
		if a.Args["ConnectionID"] != "0" {
			return nil, &upnp.Fault{Code: 706, Description: "Invalid connection reference"}
		}
		return []upnp.Arg{
			{Name: "RcsID", Value: "-1"},
			{Name: "AVTransportID", Value: "-1"},
			{Name: "ProtocolInfo"},
			{Name: "PeerConnectionManager"},
			{Name: "PeerConnectionID", Value: "-1"},
			{Name: "Direction", Value: "Output"},
			{Name: "Status", Value: "OK"},
		}, nil
	}
	return nil, upnp.ErrInvalidAction
}

// receiverRegistrarAction authorizes every device, as Windows Media
// Player and Xbox ask.
func receiverRegistrarAction(a upnp.Action) ([]upnp.Arg, *upnp.Fault) {
	switch a.Name {
	case "IsAuthorized", "IsValidated":
		return []upnp.Arg{{Name: "Result", Value: "1"}}, nil
	case "RegisterDevice":
		return []upnp.Arg{{Name: "RegistrationRespMsg"}}, nil
	}
	return nil, upnp.ErrInvalidAction
}

// sourceProtocolInfo lists what the media server serves: the MIME types
// of the generic profile's containers and of its images.
func (p *plugin) sourceProtocolInfo() string {
	_, set, _, _ := p.state()
	g := set.Generic()
	seen := map[string]bool{}
	var out []string
	add := func(mime string) {
		if !seen[mime] {
			seen[mime] = true
			out = append(out, "http-get:*:"+mime+":*")
		}
	}
	caps := g.ClientCapabilities()
	for _, d := range caps.GetDirectPlay() {
		kind := mediaKindName(d.GetKind())
		for c := range strings.SplitSeq(d.GetContainer(), ",") {
			add(g.MimeType(c, kind))
		}
	}
	for _, t := range caps.GetTranscoding() {
		add(g.MimeType(t.GetContainer(), mediaKindName(t.GetKind())))
	}
	add(profile.DefaultMimeType("jpg", "image"))
	return strings.Join(out, ",")
}

// xmlText escapes text for XML.
func xmlText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// icon is an icon of the media server.
type icon struct {
	File, Type string
	Size       int
}

var icons = []icon{
	{"48.png", "image/png", 48},
	{"120.png", "image/png", 120},
	{"48.jpg", "image/jpeg", 48},
	{"120.jpg", "image/jpeg", 120},
}

// iconData draws the icons once: a white play triangle on Mavio's purple.
var iconData = sync.OnceValue(func() map[string][]byte {
	out := map[string][]byte{}
	for _, ic := range icons {
		img := image.NewRGBA(image.Rect(0, 0, ic.Size, ic.Size))
		bg, fg := color.RGBA{0x5b, 0x3c, 0xc4, 0xff}, color.RGBA{0xff, 0xff, 0xff, 0xff}
		n := float64(ic.Size)
		for y := range ic.Size {
			for x := range ic.Size {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				// The triangle's corners: (0.35, 0.25), (0.35, 0.75), (0.75, 0.5).
				in := fx >= 0.35*n && fy >= 0.25*n+(fx-0.35*n)*0.625 && fy <= 0.75*n-(fx-0.35*n)*0.625
				if in {
					img.Set(x, y, fg)
				} else {
					img.Set(x, y, bg)
				}
			}
		}
		var b bytes.Buffer
		if ic.Type == "image/png" {
			_ = png.Encode(&b, img)
		} else {
			_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
		}
		out[ic.File] = b.Bytes()
	}
	return out
})

func serveIcon(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	data, ok := iconData()[file]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(file, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}
