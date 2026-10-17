package rpc

import (
	"context"

	"connectrpc.com/connect"
	"golang.org/x/text/language"
)

// wantedLanguages are the languages the caller reads, most wanted first,
// from the Accept-Language header clients send with their language; none
// when the header is missing or cannot be read.
func wantedLanguages(ctx context.Context) []language.Tag {
	info, ok := connect.CallInfoForHandlerContext(ctx)
	if !ok {
		return nil
	}
	tags, _, err := language.ParseAcceptLanguage(info.RequestHeader().Get("Accept-Language"))
	if err != nil {
		return nil
	}
	return tags
}
