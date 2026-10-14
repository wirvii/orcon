// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/netip"
	"net/url"
	"path"
	"strings"
)

// crossValidate checks the rules that involve more than one key (R-CONF-10).
// It skips a rule when a key it reads is itself invalid: that key already has its own error.
func (l *loader) crossValidate() {
	c := l.cfg
	for _, r := range []func(*loader, *Config){
		(*loader).checkPublicURL,
		(*loader).checkDriverAndRole,
		(*loader).checkBlobs,
		(*loader).checkBackupTargets,
		(*loader).checkMasterKey,
		(*loader).checkTLS,
		(*loader).checkOTLP,
		(*loader).checkInlineSecrets,
	} {
		r(l, c)
	}
}

func (c *Config) invalid(paths ...string) bool {
	for _, p := range paths {
		if e, ok := c.entries[p]; ok && e.Invalid {
			return true
		}
	}
	return false
}

func (c *Config) set(path string) bool { return !c.Value(path).IsZero() }

// isLoopbackHost reports whether host is localhost or a loopback address.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	return err == nil && addr.IsLoopback()
}

func (l *loader) checkPublicURL(c *Config) {
	if c.invalid("publicUrl") {
		return
	}
	u, err := url.Parse(c.Value("publicUrl").Text())
	if err != nil || u.Scheme != "http" || isLoopbackHost(u.Hostname()) {
		return
	}
	if c.set("http.tls.certificateFile") && c.set("http.tls.keyFile") {
		return
	}
	l.errorf("publicUrl", "http:// with the non-loopback host %q: sessions require HTTPS (set http.tls.* or terminate TLS in a proxy and use https://)", u.Hostname())
}

func (l *loader) checkDriverAndRole(c *Config) {
	if c.invalid("store.driver", "role") {
		return
	}
	if c.Value("store.driver").Text() == "sqlite" && c.Role() != RoleStandalone {
		l.errorf("store.driver", "sqlite needs role %q because only one process can own the database; the role is %q (use store.driver postgres for separate roles)", RoleStandalone, c.Role())
	}
	if c.Value("store.driver").Text() == "postgres" && !c.set("store.postgres.dsn") {
		l.errorf("store.postgres.dsn", "required when store.driver is postgres; set it, or store.postgres.dsnFile")
	}
}

func (l *loader) checkBlobs(c *Config) {
	if c.invalid("blobs.backend", "store.driver") {
		return
	}
	backend, driver := c.Value("blobs.backend").Text(), c.Value("store.driver").Text()
	switch {
	case backend == "filesystem" && driver == "postgres":
		l.warnf("blobs.backend", "filesystem with store.driver postgres needs a path shared by every process (a shared volume), or blobs are missing on the other processes")
	case backend == "postgres" && driver != "postgres":
		l.errorf("blobs.backend", "postgres stores blobs in the PostgreSQL database, so store.driver must be postgres")
	case backend == "s3" && !c.set("blobs.s3.bucket"):
		l.errorf("blobs.s3.bucket", "required when blobs.backend is s3")
	}
}

func (l *loader) checkBackupTargets(c *Config) {
	if c.invalid("backup.target", "backup.keys.target") {
		return
	}
	target, keys := c.Value("backup.target").Text(), c.Value("backup.keys.target").Text()
	if c.set("backup.schedule") && target == "" {
		l.errorf("backup.schedule", "a schedule needs backup.target to say where the backups go")
	}
	if target == "" || keys == "" {
		return
	}
	switch relation := compareTargets(target, keys); relation {
	case targetsSame:
		l.errorf("backup.keys.target", "equals backup.target: the key artifact must go to a different directory or bucket than the data artifact")
	case targetsKeysInside:
		l.errorf("backup.keys.target", "is inside backup.target: the key artifact must go to a different directory or bucket than the data artifact")
	case targetsDataInside:
		l.errorf("backup.keys.target", "contains backup.target: the key artifact must go to a different directory or bucket than the data artifact")
	case targetsApart:
	}
}

type targetRelation int

const (
	targetsApart targetRelation = iota
	targetsSame
	targetsKeysInside
	targetsDataInside
)

func compareTargets(data, keys string) targetRelation {
	d, k := parseTarget(data), parseTarget(keys)
	if d.root != k.root {
		return targetsApart
	}
	switch {
	case d.path == k.path:
		return targetsSame
	case within(k.path, d.path):
		return targetsKeysInside
	case within(d.path, k.path):
		return targetsDataInside
	default:
		return targetsApart
	}
}

type target struct{ root, path string }

// parseTarget splits a backup URI into its root (scheme and host) and a cleaned path.
func parseTarget(raw string) target {
	u, err := url.Parse(raw)
	if err != nil {
		return target{root: raw}
	}
	p := path.Clean("/" + u.Path)
	return target{root: u.Scheme + "://" + strings.ToLower(u.Host), path: p}
}

func within(child, parent string) bool {
	if parent == "/" {
		return child != "/"
	}
	return strings.HasPrefix(child, parent+"/")
}

func (l *loader) checkMasterKey(c *Config) {
	if c.invalid("keys.master.provider") {
		return
	}
	need := map[string][]string{
		"env":           {"keys.master.key"},
		"awskms":        {"keys.master.awskms.keyArn"},
		"gcpkms":        {"keys.master.gcpkms.keyName"},
		"azurekv":       {"keys.master.azurekv.keyId"},
		"vault-transit": {"keys.master.vaultTransit.address", "keys.master.vaultTransit.mount", "keys.master.vaultTransit.keyName", "keys.master.vaultTransit.token"},
	}
	provider := c.Value("keys.master.provider").Text()
	if provider == "env" {
		l.warnf("keys.master.provider", "env keeps the master key in the process environment, where it can leak through process listings and crash dumps; prefer a file (keys.master.keyFile) or a KMS")
	}
	for _, p := range need[provider] {
		if !c.set(p) {
			l.errorf(p, "required when keys.master.provider is %s", provider)
		}
	}
}

func (l *loader) checkTLS(c *Config) {
	cert, key := c.set("http.tls.certificateFile"), c.set("http.tls.keyFile")
	if cert != key {
		missing := "http.tls.keyFile"
		if !cert {
			missing = "http.tls.certificateFile"
		}
		l.errorf(missing, "http.tls.certificateFile and http.tls.keyFile must be set together")
	}
}

func (l *loader) checkOTLP(c *Config) {
	for _, signal := range []string{"metrics", "traces", "logs"} {
		key := "observability." + signal + ".otlp.enabled"
		if c.invalid(key) || !c.Value(key).Bool() {
			continue
		}
		if !c.set("observability.otlp.endpoint") {
			l.errorf("observability.otlp.endpoint", "required when %s is true", key)
		}
	}
}

// checkInlineSecrets warns when a secret is written in a configuration file other users can read.
func (l *loader) checkInlineSecrets(c *Config) {
	if l.in.GOOS == "windows" || len(c.inlineSecrets) == 0 || !othersCanRead(c.fileMode) {
		return
	}
	for _, p := range c.inlineSecrets {
		l.warnf(p, "the secret is written in %s, which other users can read; restrict the file (chmod 0600) or move the secret to a file and use %s", c.file, FilePath(p))
	}
}
