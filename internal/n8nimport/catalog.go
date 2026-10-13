// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Effort is the estimated cost class of a connector (ADR-0037 section 4).
type Effort string

// Efforts and their weights: declarative 1, native 3, new trigger family 3.
const (
	// EffortDeclarative is an HTTP connector written in the declarative format.
	EffortDeclarative Effort = "DECLARATIVE"
	// EffortNative is a Go connector for a protocol the declarative format cannot speak.
	EffortNative Effort = "NATIVE"
	// EffortTriggerFamily is a connector that needs a trigger family Orcon lacks.
	EffortTriggerFamily Effort = "TRIGGER_FAMILY"
)

// Weight returns the cost of the class.
func (e Effort) Weight() int {
	switch e {
	case EffortNative, EffortTriggerFamily:
		return 3
	case EffortDeclarative:
		return 1
	default:
		return 1
	}
}

// UnitMap declares that the importer handles one unit and, optionally, the
// Orcon operation it becomes.
type UnitMap struct {
	Unit
	// OrconOperation is the Orcon operation name, resource.verb (ADR-0029
	// section 4). It is empty in a planned catalog, where the connector
	// author still has to choose it.
	OrconOperation string
}

// Connector is a package of the catalog and the n8n units it replaces.
type Connector struct {
	ID     string
	Effort Effort
	// OrconType is the Orcon node type of the connector's action node
	// (ADR-0029 section 3). It is empty while the connector is only planned:
	// a planned connector counts for coverage but maps nothing.
	OrconType string
	Units     []UnitMap
	// Credentials maps n8n credential types to Orcon credential types
	// (R-IMP-02). A type missing here is reported on import.
	Credentials map[string]string
}

// Catalog is what the importer can map besides the core table: the n8n
// units each connector declares. The zero value is an empty catalog.
type Catalog struct {
	Connectors []Connector

	byUnit map[Unit]catalogEntry
}

type catalogEntry struct {
	connector string
	operation string
}

// NewCatalog indexes connectors. It rejects a unit declared twice, because the
// importer must never choose between two mappings.
func NewCatalog(connectors ...Connector) (*Catalog, error) {
	c := &Catalog{Connectors: slices.Clone(connectors), byUnit: map[Unit]catalogEntry{}}
	for _, k := range c.Connectors {
		if k.ID == "" {
			return nil, errors.New("n8nimport: a connector has no id")
		}
		for _, u := range k.Units {
			if u.Type == "" || u.TypeVersion == "" {
				return nil, fmt.Errorf("n8nimport: connector %q declares a unit without type or typeVersion", k.ID)
			}
			if prev, dup := c.byUnit[u.Unit]; dup {
				return nil, fmt.Errorf("n8nimport: unit %s is declared by %q and %q", u.Unit, prev.connector, k.ID)
			}
			c.byUnit[u.Unit] = catalogEntry{connector: k.ID, operation: u.OrconOperation}
		}
	}
	return c, nil
}

// Lookup returns the connector and Orcon operation that handle a unit.
func (c *Catalog) Lookup(u Unit) (connector, operation string, ok bool) {
	if c == nil {
		return "", "", false
	}
	e, ok := c.byUnit[u]
	return e.connector, e.operation, ok
}

// Has reports whether any unit of the n8n type is declared.
func (c *Catalog) Has(nodeType string) bool {
	if c == nil {
		return false
	}
	for u := range c.byUnit {
		if u.Type == nodeType {
			return true
		}
	}
	return false
}

// HasVersion reports whether any unit of the n8n type and typeVersion is
// declared.
func (c *Catalog) HasVersion(nodeType, typeVersion string) bool {
	if c == nil {
		return false
	}
	for u := range c.byUnit {
		if u.Type == nodeType && u.TypeVersion == typeVersion {
			return true
		}
	}
	return false
}

// Connector returns the connector that declares the n8n type.
func (c *Catalog) Connector(nodeType string) (Connector, bool) {
	if c == nil {
		return Connector{}, false
	}
	for u, e := range c.byUnit {
		if u.Type != nodeType {
			continue
		}
		for _, k := range c.Connectors {
			if k.ID == e.connector {
				return k, true
			}
		}
	}
	return Connector{}, false
}

// Credential returns the Orcon credential type declared for an n8n one.
func (c *Catalog) Credential(n8nType string) (string, bool) {
	if c == nil {
		return "", false
	}
	for _, k := range c.Connectors {
		if o, ok := k.Credentials[n8nType]; ok {
			return o, true
		}
	}
	return "", false
}

// Merge returns a catalog with the connectors of both. A unit declared by
// both is an error.
func Merge(a, b *Catalog) (*Catalog, error) {
	var all []Connector
	if a != nil {
		all = append(all, a.Connectors...)
	}
	if b != nil {
		all = append(all, b.Connectors...)
	}
	return NewCatalog(all...)
}

// ParseCatalog reads a catalog file:
//
//	{"connectors":[{"id":"slack","effort":"DECLARATIVE",
//	  "credentials":{"slackApi":"slack.token"},
//	  "units":[{"type":"n8n-nodes-base.slack","typeVersion":"2.2",
//	            "resource":"message","operation":"post","orcon":"message.send"}]}]}
func ParseCatalog(data []byte) (*Catalog, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("n8nimport: parse catalog: %w", err)
	}
	list, ok := v.Get("connectors")
	if !ok || list.Kind() != jsonvalue.KindArray {
		return nil, errors.New("n8nimport: the catalog has no connectors array")
	}
	var cs []Connector
	for i, item := range list.Items() {
		k := Connector{ID: strMember(item, "id"), Effort: Effort(strMember(item, "effort")), OrconType: strMember(item, "orconType")}
		if !slices.Contains([]Effort{EffortDeclarative, EffortNative, EffortTriggerFamily}, k.Effort) {
			return nil, fmt.Errorf("n8nimport: connector %d (%q): effort %q is not DECLARATIVE, NATIVE or TRIGGER_FAMILY", i, k.ID, k.Effort)
		}
		if creds, ok := item.Get("credentials"); ok {
			k.Credentials = map[string]string{}
			for _, m := range creds.Members() {
				s, _ := m.Value.Str()
				k.Credentials[m.Key] = s
			}
		}
		units, _ := item.Get("units")
		for _, u := range units.Items() {
			k.Units = append(k.Units, UnitMap{
				Unit: Unit{
					Type: strMember(u, "type"), TypeVersion: strMember(u, "typeVersion"),
					Resource: strMember(u, "resource"), Operation: strMember(u, "operation"),
				},
				OrconOperation: strMember(u, "orcon"),
			})
		}
		cs = append(cs, k)
	}
	return NewCatalog(cs...)
}

// Encode writes the catalog in the format [ParseCatalog] reads, with
// connectors and units in a stable order.
func (c *Catalog) Encode() ([]byte, error) {
	conns := slices.Clone(c.Connectors)
	slices.SortFunc(conns, func(a, b Connector) int { return strings.Compare(a.ID, b.ID) })
	list := make([]jsonvalue.Value, 0, len(conns))
	for i := range conns {
		v, err := encodeConnector(&conns[i])
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	root, err := jsonvalue.Object(jsonvalue.Member{Key: "connectors", Value: jsonvalue.Array(list...)})
	if err != nil {
		return nil, fmt.Errorf("n8nimport: encode catalog: %w", err)
	}
	return jsonvalue.Marshal(root)
}

func encodeConnector(k *Connector) (jsonvalue.Value, error) {
	units := slices.Clone(k.Units)
	slices.SortFunc(units, func(a, b UnitMap) int { return strings.Compare(a.String(), b.String()) })
	us := make([]jsonvalue.Value, 0, len(units))
	for i := range units {
		v, err := encodeUnit(&units[i])
		if err != nil {
			return jsonvalue.Value{}, err
		}
		us = append(us, v)
	}
	ms := []jsonvalue.Member{
		{Key: "id", Value: jsonvalue.String(k.ID)},
		{Key: "effort", Value: jsonvalue.String(string(k.Effort))},
	}
	if k.OrconType != "" {
		ms = append(ms, jsonvalue.Member{Key: "orconType", Value: jsonvalue.String(k.OrconType)})
	}
	if len(k.Credentials) > 0 {
		keys := make([]string, 0, len(k.Credentials))
		for key := range k.Credentials {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		cm := make([]jsonvalue.Member, 0, len(keys))
		for _, key := range keys {
			cm = append(cm, jsonvalue.Member{Key: key, Value: jsonvalue.String(k.Credentials[key])})
		}
		obj, err := jsonvalue.Object(cm...)
		if err != nil {
			return jsonvalue.Value{}, fmt.Errorf("n8nimport: encode catalog: %w", err)
		}
		ms = append(ms, jsonvalue.Member{Key: "credentials", Value: obj})
	}
	ms = append(ms, jsonvalue.Member{Key: "units", Value: jsonvalue.Array(us...)})
	obj, err := jsonvalue.Object(ms...)
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("n8nimport: encode catalog: %w", err)
	}
	return obj, nil
}

func encodeUnit(u *UnitMap) (jsonvalue.Value, error) {
	ms := []jsonvalue.Member{
		{Key: "type", Value: jsonvalue.String(u.Type)},
		{Key: "typeVersion", Value: jsonvalue.String(u.TypeVersion)},
	}
	if u.Resource != "" {
		ms = append(ms, jsonvalue.Member{Key: "resource", Value: jsonvalue.String(u.Resource)})
	}
	if u.Operation != "" {
		ms = append(ms, jsonvalue.Member{Key: "operation", Value: jsonvalue.String(u.Operation)})
	}
	if u.OrconOperation != "" {
		ms = append(ms, jsonvalue.Member{Key: "orcon", Value: jsonvalue.String(u.OrconOperation)})
	}
	obj, err := jsonvalue.Object(ms...)
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("n8nimport: encode catalog: %w", err)
	}
	return obj, nil
}
