// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"strings"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ResponseFormat says how orcon.http@1 turns the body of a successful response
// into the item it emits. The names and meaning follow the response format of
// the n8n HTTP Request node, so that an imported flow behaves the same.
type ResponseFormat string

// The response formats of the node parameter "responseFormat".
const (
	// FormatAutodetect picks the format from the Content-Type of the response
	// (the default).
	FormatAutodetect ResponseFormat = "autodetect"
	// FormatJSON parses the body as JSON; a body that is not JSON fails the node.
	FormatJSON ResponseFormat = "json"
	// FormatText emits {"data": "<body>"}, whatever the Content-Type.
	FormatText ResponseFormat = "text"
	// FormatFile emits the body as a binary value, whatever the Content-Type.
	FormatFile ResponseFormat = "file"
)

// Valid reports whether f is one of the four formats.
func (f ResponseFormat) Valid() bool {
	switch f {
	case FormatAutodetect, FormatJSON, FormatText, FormatFile:
		return true
	}
	return false
}

// MaxInlineBodyBytes caps a body kept inside an item: the 256 KiB inline
// threshold of ADR-0016 section 1. The blob store that ADR-0016 section 1 names
// for larger and binary data is not wired into the node yet, so a text body
// beyond the cap is cut at the cap (with "truncated": true and the real size)
// and a binary body beyond it fails the node with PayloadTooLarge. The egress
// client caps every response before this point (ADR-0029 section 8).
const MaxInlineBodyBytes = 256 << 10

// binaryProperty is the name of the binary value of a file response, the n8n
// default of its "put output in field" option.
const binaryProperty = "data"

// decoder turns response bodies into values for one send.
type decoder struct {
	format ResponseFormat
	mask   *masker
}

// decode implements declarative.Options.Decode. A JSON body is parsed as it is;
// every other body becomes a value of the shapes the format names. The secrets
// of the credential are masked in the raw bytes before they are cut or encoded,
// so that neither a truncated text nor a base64 text can hide one.
func (d decoder) decode(resp declarative.HTTPResponse) (jsonvalue.Value, error) {
	kind := d.format
	if kind == FormatAutodetect {
		kind = detect(resp)
	}
	switch kind {
	case FormatJSON:
		v, err := jsonvalue.Parse(resp.Body)
		if err != nil {
			if d.format == FormatAutodetect {
				return d.text(resp)
			}
			return jsonvalue.Value{}, &declarative.CallError{Class: declarative.ClassPermanent, Code: "ProviderError", Status: resp.Status, Message: "the response is not JSON"}
		}
		return v, nil
	case FormatText:
		return d.text(resp)
	case FormatAutodetect, FormatFile:
		return d.file(resp)
	}
	return d.file(resp)
}

// detect picks the format of an autodetected response from its Content-Type. A
// response without one is JSON when it parses, text when it is valid text, and
// a file otherwise.
func detect(resp declarative.HTTPResponse) ResponseFormat {
	media := mediaType(resp)
	switch {
	case media == "":
		if _, err := jsonvalue.Parse(resp.Body); err == nil {
			return FormatJSON
		}
		if utf8.Valid(resp.Body) && !bytes.Contains(resp.Body, []byte{0}) {
			return FormatText
		}
		return FormatFile
	case media == "application/json" || strings.HasSuffix(media, "+json"):
		return FormatJSON
	case strings.HasPrefix(media, "text/") || textual(media):
		return FormatText
	default:
		return FormatFile
	}
}

// textual lists the media types that are text although they are not text/*.
func textual(media string) bool {
	switch media {
	case "application/xml", "application/xhtml+xml", "application/javascript", "application/x-javascript",
		"application/ecmascript", "application/x-www-form-urlencoded", "application/yaml", "application/x-yaml",
		"application/csv", "application/sql", "application/graphql":
		return true
	}
	return strings.HasSuffix(media, "+xml") || strings.HasSuffix(media, "+yaml")
}

// mediaType is the media type of the response without parameters, lower case;
// empty when the header is missing or malformed.
func mediaType(resp declarative.HTTPResponse) string {
	v := header(resp, "content-type")
	if v == "" {
		return ""
	}
	m, _, err := mime.ParseMediaType(v)
	if err != nil {
		m, _, _ = strings.Cut(v, ";")
		return strings.ToLower(strings.TrimSpace(m))
	}
	return m
}

func header(resp declarative.HTTPResponse, name string) string {
	for k, vs := range resp.Header {
		if strings.EqualFold(k, name) && len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

// text emits {"data": "<body>"}: the body as a UTF-8 string, cut at the inline
// cap on a character boundary.
func (d decoder) text(resp declarative.HTTPResponse) (jsonvalue.Value, error) {
	body := d.masked(resp.Body)
	size := int64(len(resp.Body))
	truncated := len(body) > MaxInlineBodyBytes
	if truncated {
		body = body[:MaxInlineBodyBytes]
	}
	s := strings.ToValidUTF8(string(body), "�")
	members := []jsonvalue.Member{{Key: "data", Value: jsonvalue.String(s)}}
	if truncated {
		members = append(members,
			jsonvalue.Member{Key: "truncated", Value: jsonvalue.Bool(true)},
			jsonvalue.Member{Key: "size", Value: jsonvalue.Int(size)})
	}
	return jsonvalue.Object(members...)
}

// file emits {"binary": {"data": {"data": "<base64>", "mimeType", "fileSize"}}},
// the shape n8n gives a binary property. A body beyond the inline cap fails the
// node: a cut file is not a file.
func (d decoder) file(resp declarative.HTTPResponse) (jsonvalue.Value, error) {
	if len(resp.Body) > MaxInlineBodyBytes {
		return jsonvalue.Value{}, &declarative.CallError{
			Class: declarative.ClassPermanent, Code: "PayloadTooLarge", Status: resp.Status,
			Message: fmt.Sprintf("a binary response is kept inline up to %d bytes and this one has %d; blob storage is not available to the node yet", MaxInlineBodyBytes, len(resp.Body)),
		}
	}
	media := mediaType(resp)
	if media == "" {
		media = "application/octet-stream"
	}
	members := []jsonvalue.Member{
		{Key: "data", Value: jsonvalue.String(base64.StdEncoding.EncodeToString(d.masked(resp.Body)))},
		{Key: "mimeType", Value: jsonvalue.String(media)},
		{Key: "fileSize", Value: jsonvalue.Int(int64(len(resp.Body)))},
	}
	if name := fileName(resp); name != "" {
		members = append(members, jsonvalue.Member{Key: "fileName", Value: jsonvalue.String(d.mask.text(name))})
	}
	inner, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	props, err := jsonvalue.Object(jsonvalue.Member{Key: binaryProperty, Value: inner})
	if err != nil {
		return jsonvalue.Value{}, err
	}
	return jsonvalue.Object(jsonvalue.Member{Key: "binary", Value: props})
}

// fileName reads the name a Content-Disposition header gives the file.
func fileName(resp declarative.HTTPResponse) string {
	v := header(resp, "content-disposition")
	if v == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(v)
	if err != nil {
		return ""
	}
	return params["filename"]
}

// masked returns the body with every secret of the credential replaced. A nil
// masker returns the body as it is.
func (d decoder) masked(b []byte) []byte {
	if d.mask == nil {
		return b
	}
	return []byte(d.mask.text(string(b)))
}
