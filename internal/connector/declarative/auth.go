// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// applyAuth writes the credential of the connector into req (ADR-0030 §4). It
// runs last, after the URL, headers and body of the request are final, because
// AWS_SIG_V4 signs them. Credential values go only into headers or the query;
// they never reach an error message or a result.
func applyAuth(a Auth, creds map[string]string, req *HTTPRequest, now time.Time) error {
	switch a.Type {
	case "", AuthNone:
		return nil
	case AuthAPIKey:
		return applyAPIKey(a, creds, req)
	case AuthBasic:
		user, err := credential(creds, "username")
		if err != nil {
			return err
		}
		pass, err := credential(creds, "password")
		if err != nil {
			return err
		}
		setHeader(req, "Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
		return nil
	case AuthBearer:
		token, err := credential(creds, "token")
		if err != nil {
			return err
		}
		setHeader(req, "Authorization", "Bearer "+token)
		return nil
	case AuthCustomHeaders:
		// Header templates are rendered by the caller, which owns the scope;
		// see [Execute]. Nothing else to do here.
		return nil
	case AuthAWSSigV4:
		return signV4(a, creds, req, now)
	default:
		return fmt.Errorf("%w: authentication scheme %s", ErrUnsupported, a.Type)
	}
}

func applyAPIKey(a Auth, creds map[string]string, req *HTTPRequest) error {
	field := a.Field
	if field == "" {
		field = "apiKey"
	}
	key, err := credential(creds, field)
	if err != nil {
		return err
	}
	value := a.Prefix + key
	switch a.In {
	case "HEADER":
		setHeader(req, a.Name, value)
	case "COOKIE":
		addHeader(req, "Cookie", a.Name+"="+value)
	case "QUERY":
		u, err := url.Parse(req.URL)
		if err != nil {
			return fmt.Errorf("%w: request URL", ErrTemplate)
		}
		q := u.Query()
		q.Set(a.Name, value)
		u.RawQuery = q.Encode()
		req.URL = u.String()
	default:
		return fmt.Errorf("%w: API_KEY placement %q", ErrUnsupported, a.In)
	}
	return nil
}

// credential reads one field. The error names the field, never its value.
func credential(creds map[string]string, field string) (string, error) {
	v, ok := creds[field]
	if !ok || v == "" {
		return "", fmt.Errorf("%w: credential field %q is missing", ErrMissingField, field)
	}
	return v, nil
}

// ErrMissingField is returned when a credential field the scheme needs is absent.
var ErrMissingField = errors.New("declarative: credential")

// ErrUnsupported is returned for a scheme or placement the interpreter does
// not run yet.
var ErrUnsupported = errors.New("declarative: unsupported")

func setHeader(req *HTTPRequest, name, value string) {
	if req.Header == nil {
		req.Header = map[string][]string{}
	}
	req.Header[name] = []string{value}
}

func addHeader(req *HTTPRequest, name, value string) {
	if req.Header == nil {
		req.Header = map[string][]string{}
	}
	req.Header[name] = append(req.Header[name], value)
}

// signV4 signs the request with AWS Signature Version 4 for the configured
// service and region. The body is the one that will be sent.
func signV4(a Auth, creds map[string]string, req *HTTPRequest, now time.Time) error {
	accessKey, err := credential(creds, "accessKeyId")
	if err != nil {
		return err
	}
	secret, err := credential(creds, "secretAccessKey")
	if err != nil {
		return err
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		return fmt.Errorf("%w: request URL", ErrTemplate)
	}
	now = now.UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	headers := map[string]string{"host": u.Host, "x-amz-date": amzDate}
	if token, ok := creds["sessionToken"]; ok && token != "" {
		headers["x-amz-security-token"] = token
	}
	names := make([]string, 0, len(headers))
	for n := range headers {
		names = append(names, n)
	}
	sort.Strings(names)
	var canonicalHeaders strings.Builder
	for _, n := range names {
		canonicalHeaders.WriteString(n + ":" + strings.TrimSpace(headers[n]) + "\n")
	}
	signedHeaders := strings.Join(names, ";")

	payload := sha256.Sum256(req.Body)
	canonicalURI := u.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery(u.Query()),
		canonicalHeaders.String(),
		signedHeaders,
		hex.EncodeToString(payload[:]),
	}, "\n")

	scope := strings.Join([]string{dateStamp, a.Region, a.Service, "aws4_request"}, "/")
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, scope, hex.EncodeToString(requestHash[:])}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, a.Region)
	kService := hmacSHA256(kRegion, a.Service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	setHeader(req, "X-Amz-Date", amzDate)
	if t, ok := headers["x-amz-security-token"]; ok {
		setHeader(req, "X-Amz-Security-Token", t)
	}
	setHeader(req, "Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signedHeaders, signature))
	return nil
}

func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, awsEscape(k)+"="+awsEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

func awsEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
