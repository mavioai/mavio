package xmlrpc

import (
	"errors"
	"testing"
)

func TestEncodeCall(t *testing.T) {
	got, err := EncodeCall("load.start", "", "a&b", 3, []byte("hi"), []string{"x"}, map[string]string{"b": "2", "a": "1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="utf-8"?><methodCall><methodName>load.start</methodName><params>` +
		`<param><value><string></string></value></param>` +
		`<param><value><string>a&amp;b</string></value></param>` +
		`<param><value><int>3</int></value></param>` +
		`<param><value><base64>aGk=</base64></value></param>` +
		`<param><value><array><data><value><string>x</string></value></data></array></value></param>` +
		`<param><value><struct><member><name>a</name><value><string>1</string></value></member>` +
		`<member><name>b</name><value><string>2</string></value></member></struct></value></param>` +
		`</params></methodCall>`
	if string(got) != want {
		t.Errorf("EncodeCall() = %s, want = %s", got, want)
	}
	if _, err := EncodeCall("m", 1.5); err == nil {
		t.Errorf("EncodeCall(float) error = nil, want unhandled type")
	}
	if got, _ := EncodeCall("system.client_version"); string(got) != `<?xml version="1.0" encoding="utf-8"?><methodCall><methodName>system.client_version</methodName></methodCall>` {
		t.Errorf("EncodeCall() without params = %s", got)
	}
}

func TestDecodeResponse(t *testing.T) {
	body := `<?xml version="1.0"?><methodResponse><params><param><value><array><data>
		<value><array><data><value><string>HASH</string></value><value><i8>42</i8></value><value>untyped</value></data></array></value>
		<value><struct><member><name>gid</name><value><string>2089b05ecca3d829</string></value></member></struct></value>
		</data></array></value></param></params></methodResponse>`
	v, err := DecodeResponse([]byte(body), "rTorrent")
	if err != nil {
		t.Fatal(err)
	}
	rows := v.Values()
	if len(rows) != 2 {
		t.Fatalf("Values() = %d, want 2", len(rows))
	}
	row := rows[0].Values()
	if row[0].Str() != "HASH" || row[1].Int64() != 42 || row[2].Str() != "untyped" {
		t.Errorf("row = %q, %d, %q, want HASH, 42, untyped", row[0].Str(), row[1].Int64(), row[2].Str())
	}
	if m := rows[1].Member("gid"); m == nil || m.Str() != "2089b05ecca3d829" {
		t.Errorf("Member(gid) = %v, want the GID", m)
	}
	if rows[1].Member("missing") != nil || rows[0].Member("gid") != nil || rows[1].Values() != nil {
		t.Errorf("missing members and values are not nil")
	}

	empty, err := DecodeResponse([]byte(`<methodResponse><params></params></methodResponse>`), "Aria2")
	if err != nil || empty.Str() != "" {
		t.Errorf("DecodeResponse(no params) = %v, %v, want an empty value", empty, err)
	}
}

func TestDecodeFault(t *testing.T) {
	body := `<methodResponse><fault><value><struct>
		<member><name>faultCode</name><value><int>-501</int></value></member>
		<member><name>faultString</name><value><string>Could not find info-hash.</string></value></member>
		</struct></value></fault></methodResponse>`
	_, err := DecodeResponse([]byte(body), "rTorrent")
	var f *Fault
	if !errors.As(err, &f) || f.Code != -501 || f.Message != "Could not find info-hash." {
		t.Fatalf("DecodeResponse() error = %v, want the fault", err)
	}
	if want := "rTorrent returned error code -501: Could not find info-hash."; err.Error() != want {
		t.Errorf("Error() = %q, want = %q", err.Error(), want)
	}
	if _, err := DecodeResponse([]byte("not xml"), "Aria2"); err == nil {
		t.Errorf("DecodeResponse(not xml) error = nil, want an error")
	}
}
