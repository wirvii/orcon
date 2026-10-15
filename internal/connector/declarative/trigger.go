// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// FamilyWebhook is the trigger family a declarative connector may declare
// (ADR-0030 §8). Only the webhook family runs in this cut; the poll and
// app-event families are refused by [Validate] until their cuts arrive.
const FamilyWebhook = "WEBHOOK"

// Verification and handshake types of a webhook trigger (ADR-0030 §8,
// ADR-0022 §5).
const (
	VerifyHMACSHA256 = "HMAC_SHA256"
	HandshakeEcho    = "VERIFY_TOKEN_ECHO"
)

// handshakeSubscribe is the mode a subscription handshake must carry; no other
// mode is valid.
const handshakeSubscribe = "subscribe"

// Trigger is one declarative trigger of a connector. A webhook trigger is
// authenticated by its signature, its subscription handshake, or both. The
// interpreter never reads the secrets: the caller passes them in, read from
// the credential fields the declaration names.
type Trigger struct {
	Family    string     `json:"family"`
	Verify    *Verify    `json:"verify,omitempty"`
	Handshake *Handshake `json:"handshake,omitempty"`
	Decode    *Decode    `json:"decode,omitempty"`
}

// Decode turns the body of an accepted webhook request into events, one per
// selected item (ADR-0030 §8). Items is a path that ends in "<key>[*]", such as
// "$.entry[*].changes[*].value.messages[*]": every array named <key> found at
// the path before it is walked, and each of its members is one event.
//
// Map builds the output of each event from three scopes: "$.item" is the
// member, "$.parent" is the object that holds the array, and "$.root" is the
// whole body. A map value may list alternatives separated by "|"; the first one
// that finds a value is used, so one key can read several message types. A path
// that finds nothing leaves its key out.
type Decode struct {
	Items string            `json:"items"`
	Map   map[string]string `json:"map"`
}

// alternatives splits a map value into its "|" alternatives, trimmed.
func alternatives(expr string) []string {
	parts := strings.Split(expr, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// splitItems separates an items path into the path of the holders of the array
// and the name of the array. It refuses a path that does not end in "<key>[*]".
func splitItems(path string) (holders, key string, err error) {
	trimmed, ok := strings.CutSuffix(path, "[*]")
	if !ok {
		return "", "", fmt.Errorf("%w: items must end in an array wildcard", ErrTemplate)
	}
	i := strings.LastIndex(trimmed, ".")
	if i < 0 || trimmed[i+1:] == "" {
		return "", "", fmt.Errorf("%w: items must name the array before [*]", ErrTemplate)
	}
	return trimmed[:i], trimmed[i+1:], nil
}

// Events returns the events of one accepted body. Each event holds the keys of
// Map whose paths found a value, so an event is a normalized object.
func (d Decode) Events(body jsonvalue.Value) ([]jsonvalue.Value, error) {
	holders, key, err := splitItems(d.Items)
	if err != nil {
		return nil, err
	}
	parents, err := selectAll(body, holders)
	if err != nil {
		return nil, err
	}
	var out []jsonvalue.Value
	for _, parent := range parents {
		arr, ok := parent.Get(key)
		if !ok {
			continue
		}
		for _, item := range arr.Items() {
			scope, err := jsonvalue.Object(
				jsonvalue.Member{Key: "item", Value: item},
				jsonvalue.Member{Key: "parent", Value: parent},
				jsonvalue.Member{Key: "root", Value: body},
			)
			if err != nil {
				return nil, err
			}
			event, err := project(scope, d.Map)
			if err != nil {
				return nil, err
			}
			out = append(out, event)
		}
	}
	return out, nil
}

// Verify is the signature check of a webhook request (HMAC_SHA256): the
// header carries the hex digest of the raw body under the secret, optionally
// after a fixed prefix such as "sha256=".
type Verify struct {
	Type   string `json:"type"`
	Header string `json:"header"`
	Prefix string `json:"prefix,omitempty"`
	// Field is the credential field holding the shared secret.
	Field string `json:"field"`
}

// Handshake is the subscription check a provider runs before it sends events
// (VERIFY_TOKEN_ECHO, as Meta does): a GET request carries a mode, a token and
// a challenge; when the mode is "subscribe" and the token equals the secret,
// the challenge is echoed back as the whole response.
type Handshake struct {
	Type string `json:"type"`
	// ModeParam, TokenParam and ChallengeParam are the query parameters.
	ModeParam      string `json:"modeParam"`
	TokenParam     string `json:"tokenParam"`
	ChallengeParam string `json:"challengeParam"`
	// Field is the credential field holding the verify token.
	Field string `json:"field"`
}

// Errors of the trigger checks. They carry no secret and no request content.
var (
	// ErrSignature means the request has no signature, or one that does not
	// match the body under the secret.
	ErrSignature = errors.New("declarative: signature does not match")
	// ErrHandshake means the subscription request is not a valid handshake for
	// the verify token.
	ErrHandshake = errors.New("declarative: handshake refused")
)

// VerifySignature checks the signature of a webhook request. The header holds the
// request headers as the transport gives them; the name is matched without
// regard to case. The comparison is constant time.
func VerifySignature(v Verify, header map[string][]string, body []byte, secret string) error {
	if secret == "" {
		return ErrSignature
	}
	got := headerValue(header, v.Header)
	if got == "" {
		return ErrSignature
	}
	if v.Prefix != "" {
		trimmed, ok := strings.CutPrefix(got, v.Prefix)
		if !ok {
			return ErrSignature
		}
		got = trimmed
	}
	sum, err := hex.DecodeString(got)
	if err != nil {
		return ErrSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	if !hmac.Equal(sum, mac.Sum(nil)) {
		return ErrSignature
	}
	return nil
}

// CheckHandshake answers a subscription request. It returns the challenge to
// echo when the mode is "subscribe" and the token equals the verify token;
// otherwise it returns [ErrHandshake].
func CheckHandshake(h Handshake, query map[string]string, verifyToken string) (string, error) {
	if verifyToken == "" || query[h.ModeParam] != handshakeSubscribe {
		return "", ErrHandshake
	}
	if !hmac.Equal([]byte(query[h.TokenParam]), []byte(verifyToken)) {
		return "", ErrHandshake
	}
	challenge := query[h.ChallengeParam]
	if challenge == "" {
		// Echoing nothing would make an empty 200 look like a confirmed address.
		return "", ErrHandshake
	}
	return challenge, nil
}

// headerValue returns the first value of the named header, or "" when absent.
func headerValue(header map[string][]string, name string) string {
	for k, values := range header {
		if strings.EqualFold(k, name) && len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}

// validTrigger checks the shape of one declared trigger.
func (v *validator) trigger(base string, t Trigger) {
	if t.Family != FamilyWebhook {
		v.addf(base+"/family", "%q is not a trigger family this interpreter runs yet", t.Family)
		return
	}
	if t.Verify == nil && t.Handshake == nil {
		v.addf(base, "a webhook trigger needs a verify or a handshake")
	}
	if t.Verify != nil {
		if t.Verify.Type != VerifyHMACSHA256 {
			v.addf(base+"/verify/type", "must be HMAC_SHA256")
		}
		if t.Verify.Header == "" {
			v.addf(base+"/verify/header", "needs the name of the signature header")
		}
		if t.Verify.Field == "" {
			v.addf(base+"/verify/field", "needs the credential field of the secret")
		}
	}
	if t.Handshake != nil {
		h := t.Handshake
		if h.Type != HandshakeEcho {
			v.addf(base+"/handshake/type", "must be VERIFY_TOKEN_ECHO")
		}
		if h.ModeParam == "" || h.TokenParam == "" || h.ChallengeParam == "" {
			v.addf(base+"/handshake", "needs modeParam, tokenParam and challengeParam")
		}
		if h.Field == "" {
			v.addf(base+"/handshake/field", "needs the credential field of the verify token")
		}
	}
	if t.Decode != nil {
		v.decode(base+"/decode", *t.Decode)
	}
}

// decode checks the items path and the event map of a webhook trigger. Each map
// alternative must read from one of the three scopes of an event.
func (v *validator) decode(base string, d Decode) {
	holders, _, err := splitItems(d.Items)
	if err != nil {
		v.addf(base+"/items", "%v", err)
	} else if _, err := parsePath(holders); err != nil {
		v.addf(base+"/items", "unsupported path: %v", err)
	}
	if len(d.Map) == 0 {
		v.addf(base+"/map", "needs at least one output member")
	}
	for _, k := range slices.Sorted(maps.Keys(d.Map)) {
		p := base + "/map/" + k
		if k == "" {
			v.addf(base+"/map", "output keys must not be empty")
		}
		for _, alt := range alternatives(d.Map[k]) {
			steps, err := parsePath(alt)
			switch {
			case err != nil:
				v.addf(p, "unsupported path: %v", err)
			case checkPlainPath(alt) != nil:
				v.addf(p, "a wildcard is not allowed in an event map")
			case len(steps) == 0 || steps[0].isIndex || !slices.Contains([]string{"item", "parent", "root"}, steps[0].key):
				v.addf(p, "a path must start with $.item, $.parent or $.root")
			}
		}
	}
}
