// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"log/slog"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// TestOfficialPackagesLoadTheirAssets: every official package built into the binary has an icon that
// passes the sanitizer and a source line, because the start fails otherwise.
func TestOfficialPackagesLoadTheirAssets(t *testing.T) {
	assets, err := officialConnectorAssets()
	if err != nil {
		t.Fatalf("official connector assets: %v", err)
	}
	if len(assets) != len(officialPackages) {
		t.Fatalf("%d assets for %d official packages", len(assets), len(officialPackages))
	}
	for _, p := range officialPackages {
		if assets[p.id].Icon == nil {
			t.Errorf("the official connector %s has no icon", p.id)
		}
	}
}

// TestCatalogListsEveryOfficialPackageNodeType: after the start records the official packages, the node
// type catalog lists every node type each of them declares, with the icon of its package, in the group the
// palette needs. A package listed in officialPackages but never recorded left the catalog without its
// nodes while the icon was loaded and never shown; this keeps that from returning silently.
func TestCatalogListsEveryOfficialPackageNodeType(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
			st, ok := env.Store.(interface {
				connector.PackageStore
				httpapi.InstalledPackages
			})
			if !ok {
				t.Fatalf("the %s store does not keep connector packages", kind)
			}
			// Recording twice is the restart of a process (or of a second replica): it changes nothing.
			for range 2 {
				if err := registerOfficialPackages(t.Context(), st, clock.System{}, env.IDs); err != nil {
					t.Fatalf("register the official packages: %v", err)
				}
			}
			assets, err := officialConnectorAssets()
			if err != nil {
				t.Fatalf("official connector assets: %v", err)
			}
			listed, err := httpapi.NewNodeCatalog(st, assets).NodeTypes(t.Context(), storetest.ScopeOf(env.Org), env.Project)
			if err != nil {
				t.Fatalf("list the node types: %v", err)
			}
			byType := map[string]int{}
			for i := range listed {
				byType[listed[i].Type]++
			}
			for _, p := range officialPackages {
				raw, err := fs.ReadFile(p.files, packageManifestFile)
				if err != nil {
					t.Fatalf("read the manifest of %s: %v", p.id, err)
				}
				m, err := connector.Parse(raw, connector.Options{Official: true})
				if err != nil {
					t.Fatalf("parse the manifest of %s: %v", p.id, err)
				}
				if len(m.Nodes) == 0 {
					t.Fatalf("the official package %s declares no node", p.id)
				}
				for _, n := range m.Nodes {
					if byType[n.Type] != 1 {
						t.Errorf("the catalog lists %s %d times, want once", n.Type, byType[n.Type])
					}
					for i := range listed {
						nt := &listed[i]
						if nt.Type != n.Type {
							continue
						}
						if nt.Icon == nil || nt.Icon.Svg == "" || nt.Icon.Source == "" {
							t.Errorf("%s has no icon in the catalog", n.Type)
						}
						wantGroup := "Apps"
						if n.Group == connector.GroupTrigger {
							wantGroup = "Triggers"
						}
						if nt.Group != wantGroup {
							t.Errorf("%s is in the group %q, want %q", n.Type, nt.Group, wantGroup)
						}
						if nt.ConnectorId == nil {
							t.Errorf("%s names no connector", n.Type)
						}
					}
				}
			}
		})
	}
}

// TestStartRecordsTheOfficialPackages: the real start (store, keys and services) leaves every official
// package installed, so the node type endpoint lists its nodes without any step by the owner.
func TestStartRecordsTheOfficialPackages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORCON_DATA_DIR", dir)
	ack := provisionMasterKey(t, dir)
	sec, err := openSecurity(t.Context(), loadTestConfig(t, "ORCON_DATA_DIR="+dir, "ORCON_MASTER_KEY_BACKUP_ACK="+ack), slog.New(slog.DiscardHandler), "test")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		if err := sec.close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	assets, err := officialConnectorAssets()
	if err != nil {
		t.Fatalf("official connector assets: %v", err)
	}
	listed, err := httpapi.NewNodeCatalog(sec.store, assets).NodeTypes(t.Context(), orgscope.Org{}, identity.ProjectID{})
	if err != nil {
		t.Fatalf("list the node types: %v", err)
	}
	for _, want := range []string{"orcon.whatsapp.whatsapp@1", "orcon.whatsapp.whatsappTrigger@1"} {
		if !slices.ContainsFunc(listed, func(nt apigen.NodeType) bool { return nt.Type == want && nt.Icon != nil }) {
			t.Errorf("the catalog after a start does not list %s with its icon", want)
		}
	}
}
