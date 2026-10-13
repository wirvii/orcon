// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/base64"
	"errors"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// productKeys returns every key of the server in resolution order. Each area
// declares its keys in its own function and file (keys_<area>.go), so a spec that
// adds keys adds a function and one line here, and never edits another area's keys.
// A computed default may read only keys that come before it in this order.
//
// Master key provider settings (SPEC-004) live in keysSecurity; the credential
// area registers further provider keys there and not in another area's function.
func productKeys() []Key {
	var all []Key
	for _, group := range [][]Key{
		keysProcess(),
		keysStore(),
		keysSecurity(),
		keysBackup(),
		keysObservability(),
	} {
		all = append(all, group...)
	}
	return all
}

// Shorthand for the role lists of the key tables.
var (
	rolesServing   = []Role{RoleStandalone, RoleAPI}
	rolesScheduler = []Role{RoleStandalone, RoleScheduler}
)

func text(en, es string) Text { return Text{EN: en, ES: es} }

// join builds a path with the separator of the host, for computed defaults.
func join(goos string, elems ...string) string {
	sep := "/"
	if goos == "windows" {
		sep = `\`
	}
	return strings.Join(elems, sep)
}

// underDataDir makes a default that is a path below dataDir.
func underDataDir(elems ...string) DefaultFunc {
	return func(env Platform, get Lookup) (string, error) {
		dir := get("dataDir")
		if dir == "" {
			return "", errors.New("dataDir is not set")
		}
		return join(env.GOOS, append([]string{strings.TrimRight(dir, `/\`)}, elems...)...), nil
	}
}

func docUnderDataDir(rel string) Text {
	return text("<dataDir>/"+rel, "<dataDir>/"+rel)
}

var (
	ageRecipientPattern = regexp.MustCompile(`^age1[0-9a-z]{20,}$`)
	cronFieldPattern    = regexp.MustCompile(`^\S+(\s+\S+){4}$`)
)

func validateAgeRecipient(v string) error {
	if !ageRecipientPattern.MatchString(v) {
		return errors.New("must be an age public key starting with age1")
	}
	return nil
}

func validateCron(v string) error {
	if !cronFieldPattern.MatchString(strings.TrimSpace(v)) {
		return errors.New("must be a cron expression with five fields (minute hour day-of-month month day-of-week), in UTC")
	}
	return nil
}

func validateCIDR(v string) error {
	if _, err := netip.ParsePrefix(v); err != nil {
		return errors.New("must be a network in CIDR form, for example 10.0.0.0/8")
	}
	return nil
}

func validateHTTPURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("must be an http:// or https:// URL")
	}
	return nil
}

func validatePublicURL(v string) error {
	if err := validateHTTPURL(v); err != nil {
		return err
	}
	u, _ := url.Parse(v) // validated just above
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("must not contain credentials, a query or a fragment")
	}
	return nil
}

func validateBackupTarget(v string) error {
	u, err := url.Parse(v)
	if err != nil {
		return errors.New("must be file:///<absolute path> or s3://<bucket>/<prefix>")
	}
	switch u.Scheme {
	case "file":
		if u.Host != "" || !strings.HasPrefix(u.Path, "/") {
			return errors.New("a file target is file:/// followed by an absolute path")
		}
	case "s3":
		if u.Host == "" {
			return errors.New("an s3 target is s3://<bucket>/<prefix>")
		}
	default:
		return errors.New("must be file:///<absolute path> or s3://<bucket>/<prefix>")
	}
	return nil
}

func validateEmail(v string) error {
	if _, err := mail.ParseAddress(v); err != nil {
		return errors.New("must be an email address")
	}
	return nil
}

func validateBase64(v string) error {
	if _, err := base64.StdEncoding.DecodeString(v); err != nil {
		return errors.New("must be base64")
	}
	return nil
}
