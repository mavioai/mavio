package jsonx

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnmarshal(t *testing.T) {
	type values struct {
		S  String  `json:"s"`
		I  Int     `json:"i"`
		L  Int64   `json:"l"`
		F  Float   `json:"f"`
		B  Bool    `json:"b"`
		T  Time    `json:"t"`
		PS *String `json:"ps"`
	}
	tests := []struct {
		name    string
		in      string
		want    values
		wantErr bool
	}{
		{
			"native", `{"s":"x","i":5,"l":57473058680,"f":1.5,"b":true,"t":"2022-12-15 19:37:29"}`,
			values{S: "x", I: 5, L: 57473058680, F: 1.5, B: true, T: Time{time.Date(2022, 12, 15, 19, 37, 29, 0, time.UTC)}},
			false,
		},
		{"strings", `{"s":12,"i":"5","l":"7","f":"2.25","b":"True"}`, values{S: "12", I: 5, L: 7, F: 2.25, B: true}, false},
		{"numbers as booleans", `{"b":0,"i":5.0,"s":false}`, values{S: "False", I: 5}, false},
		{"nulls", `{"s":null,"i":null,"b":null,"t":null,"ps":null}`, values{}, false},
		{"bad int", `{"i":"x"}`, values{}, true},
		{"int overflow", `{"i":3000000000}`, values{}, true},
		{"bad bool", `{"b":"yes"}`, values{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got values
			err := json.Unmarshal([]byte(tt.in), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.PS != nil || got.S != tt.want.S || got.I != tt.want.I || got.L != tt.want.L || got.F != tt.want.F || got.B != tt.want.B || !got.T.Equal(tt.want.T.Time) {
				t.Errorf("Unmarshal() got = %+v, want = %+v", got, tt.want)
			}
		})
	}
}
