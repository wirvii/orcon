// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/connector/authoring"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/n8nimport"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The palette groups of the node types. They are stable keys, not translated: the editor builds
// its palette from them.
const (
	groupTriggers = "Triggers"
	groupApps     = "Apps"
	groupData     = "Data"
	groupFlow     = "Flow"
)

// n8nPrefix is the namespace of the n8n core node types.
const n8nPrefix = "n8n-nodes-base."

// InstalledPackages lists the connector packages the instance has installed.
type InstalledPackages interface {
	ListPackages(ctx context.Context, after connector.PackageID, limit int) ([]connector.PackageRecord, error)
}

// ConnectorAssets are the files of an official connector package that the catalog shows: its
// brand icon, checked when the package is loaded, and its message catalogs by language.
type ConnectorAssets struct {
	// Icon is nil when the package ships none.
	Icon *authoring.Icon
	// Messages maps a language to the translated texts of the package (i18n/<language>.json).
	Messages map[string]map[string]string
}

// LoadConnectorAssets reads the icon and the message catalogs of the package whose root is
// fsys. The icon goes through [authoring.LoadIcon], so an unsafe one fails here and never reaches
// the API.
func LoadConnectorAssets(fsys fs.FS) (ConnectorAssets, error) {
	icon, err := authoring.LoadIcon(fsys)
	if err != nil {
		return ConnectorAssets{}, fmt.Errorf("load the icon: %w", err)
	}
	assets := ConnectorAssets{Icon: &icon, Messages: map[string]map[string]string{}}
	for _, lang := range []string{LocaleEnglish, LocaleSpanish} {
		raw, err := fs.ReadFile(fsys, "i18n/"+lang+".json")
		if err != nil {
			continue
		}
		var messages map[string]string
		if err := json.Unmarshal(raw, &messages); err != nil {
			return ConnectorAssets{}, fmt.Errorf("read i18n/%s.json: %w", lang, err)
		}
		assets.Messages[lang] = messages
	}
	return assets, nil
}

// NodeCatalog is the catalog of node types a project can use: the built-in types of the flow
// engine and the nodes of the installed connector packages, with names and descriptions in the
// language of the request. Packages with assets are the official ones, which carry the seal
// OFFICIAL; every other installed package is UNREVIEWED until signing exists.
type NodeCatalog struct {
	packages InstalledPackages
	assets   map[string]ConnectorAssets
}

var _ NodeTypeCatalog = (*NodeCatalog)(nil)

// NewNodeCatalog returns the catalog over the installed packages. The assets map holds, by the
// id of each official connector (for example "orcon.whatsapp"), its loaded assets.
func NewNodeCatalog(packages InstalledPackages, assets map[string]ConnectorAssets) *NodeCatalog {
	return &NodeCatalog{packages: packages, assets: assets}
}

// NodeTypes implements [NodeTypeCatalog]. The catalog is the same for every project of the
// instance: connector packages are shared (a grant decides use at publication).
func (c *NodeCatalog) NodeTypes(ctx context.Context, _ orgscope.Org, _ identity.ProjectID) ([]apigen.NodeType, error) {
	locale := LocaleFrom(ctx)
	out := builtinNodeTypes(locale)
	latest, err := c.latestPackages(ctx)
	if err != nil {
		return nil, err
	}
	for i := range latest {
		rec := &latest[i]
		m, err := connector.Parse(rec.Manifest, connector.Options{Official: true})
		if err != nil {
			return nil, fmt.Errorf("read the manifest of %s %s: %w", rec.ConnectorID, rec.Version, err)
		}
		assets, official := c.assets[rec.ConnectorID]
		for j := range m.Nodes {
			out = append(out, connectorNodeType(rec, &m, &m.Nodes[j], &assets, official, locale))
		}
	}
	return out, nil
}

// latestPackages returns the newest installed version of every connector.
func (c *NodeCatalog) latestPackages(ctx context.Context) ([]connector.PackageRecord, error) {
	best := map[string]connector.PackageRecord{}
	var after connector.PackageID
	for {
		batch, err := c.packages.ListPackages(ctx, after, 200)
		if err != nil {
			return nil, fmt.Errorf("list the installed connectors: %w", err)
		}
		for i := range batch {
			rec := batch[i]
			if rec.Status != connector.StatusInstalled {
				continue
			}
			if cur, ok := best[rec.ConnectorID]; !ok || connector.CompareVersions(rec.Version, cur.Version) > 0 {
				best[rec.ConnectorID] = rec
			}
		}
		if len(batch) < 200 {
			break
		}
		after = batch[len(batch)-1].ID
	}
	out := make([]connector.PackageRecord, 0, len(best))
	for key := range best {
		out = append(out, best[key])
	}
	slices.SortFunc(out, func(a, b connector.PackageRecord) int {
		if a.ConnectorID < b.ConnectorID {
			return -1
		}
		if a.ConnectorID > b.ConnectorID {
			return 1
		}
		return 0
	})
	return out, nil
}

// connectorNodeType describes one node of an installed package.
func connectorNodeType(rec *connector.PackageRecord, m *connector.Manifest, n *connector.Node, assets *ConnectorAssets, official bool, locale string) apigen.NodeType {
	say := func(key, fallback string) string {
		if text, ok := assets.Messages[locale][key]; ok && text != "" {
			return text
		}
		return fallback
	}
	name := say("node."+n.Name+".displayName", n.DisplayName)
	description := say("node."+n.Name+".description", say("connector.description", m.Description))
	group := groupApps
	if n.Group == connector.GroupTrigger {
		group = groupTriggers
	}
	seal := apigen.NodeTypeSealUNREVIEWED
	if official {
		seal = apigen.NodeTypeSealOFFICIAL
	}
	cid := types.UUID(rec.ID)
	nt := apigen.NodeType{
		Type: n.Type, Name: name, Description: description, Group: group, Seal: seal,
		ProofLevel: apigen.NodeTypeProofLevel(lowestProof(n)), ConnectorId: &cid,
		Operations: make([]apigen.NodeTypeOperation, 0, len(n.Operations)), N8nTypes: []string{},
	}
	for i := range n.Operations {
		op := &n.Operations[i]
		nt.Operations = append(nt.Operations, apigen.NodeTypeOperation{
			Name: op.Name, Label: say("operation."+op.Name+".displayName", op.DisplayName),
			Purity: apigen.NodeTypeOperationPurity(op.Purity), SafetyLevel: safetyOf(op),
		})
	}
	for _, t := range n.N8nTypes {
		nt.N8nTypes = append(nt.N8nTypes, t.Name)
	}
	if assets.Icon != nil {
		nt.Icon = iconBody(assets.Icon)
	}
	return nt
}

// iconBody is the inline icon of a node type.
func iconBody(icon *authoring.Icon) *apigen.NodeTypeIcon {
	body := &apigen.NodeTypeIcon{Svg: icon.SVG, Source: icon.Source}
	if icon.DarkSVG != "" {
		dark := icon.DarkSVG
		body.DarkSvg = &dark
	}
	return body
}

// safetyOf is how an EFFECT operation is protected; any other purity has none.
func safetyOf(op *connector.Operation) apigen.NodeTypeOperationSafetyLevel {
	if op.Purity != flow.PurityEffect || op.Safety == nil {
		return apigen.NONE
	}
	switch op.Safety.Level {
	case flow.SafetyIdempotencyKey:
		return apigen.IDEMPOTENCYKEY
	case flow.SafetyReconcile:
		return apigen.RECONCILE
	case flow.SafetyUnsafe:
		return apigen.UNSAFE
	case flow.SafetyReadOnly:
		return apigen.NONE
	default:
		return apigen.NONE
	}
}

// lowestProof is how far the safety contracts of the node are proven: the lowest level among its
// EFFECT operations. A node with no contract has nothing unproven.
func lowestProof(n *connector.Node) connector.ProofLevel {
	rank := map[connector.ProofLevel]int{connector.ProofDeclared: 0, connector.ProofTested: 1, connector.ProofVerified: 2}
	lowest := connector.ProofVerified
	for i := range n.Operations {
		op := &n.Operations[i]
		if op.Purity != flow.PurityEffect || op.Safety == nil {
			continue
		}
		if rank[op.Safety.ProofLevel] < rank[lowest] {
			lowest = op.Safety.ProofLevel
		}
	}
	return lowest
}

// builtin describes a node type the engine defines, with its text in each language.
type builtin struct {
	typ        string
	group      string
	name       map[string]string
	desc       map[string]string
	operations []builtinOperation
}

type builtinOperation struct {
	name   string
	label  map[string]string
	purity flow.Purity
	safety apigen.NodeTypeOperationSafetyLevel
}

// builtins are the node types of the flow engine: the format's control nodes, the data nodes and
// the HTTP node (SPEC-003 Annex B) and the AI model and agent nodes (SPEC-006). The HTTP node reads with GET and HEAD and writes with every
// other method, which stays UNSAFE without an idempotency key.
var builtins = []builtin{
	{
		typ: "orcon.http@1", group: groupApps,
		name: map[string]string{LocaleEnglish: "HTTP request", LocaleSpanish: "Solicitud HTTP"},
		desc: map[string]string{
			LocaleEnglish: "Calls an HTTP endpoint and returns the response.",
			LocaleSpanish: "Llama a un servicio HTTP y devuelve la respuesta.",
		},
		operations: []builtinOperation{
			{"request.read", map[string]string{LocaleEnglish: "Read (GET, HEAD)", LocaleSpanish: "Leer (GET, HEAD)"}, flow.PurityRead, apigen.NONE},
			{"request.write", map[string]string{LocaleEnglish: "Write (other methods)", LocaleSpanish: "Escribir (otros métodos)"}, flow.PurityEffect, apigen.UNSAFE},
		},
	},
	{
		typ: "orcon.ai.llm@1", group: groupApps,
		name: map[string]string{LocaleEnglish: "AI model", LocaleSpanish: "Modelo de IA"},
		desc: map[string]string{
			LocaleEnglish: "Sends each item to an AI model and returns its text or a checked JSON answer.",
			LocaleSpanish: "Envía cada elemento a un modelo de IA y devuelve su texto o una respuesta JSON validada.",
		},
		operations: []builtinOperation{{"model.generate", map[string]string{LocaleEnglish: "Generate an answer", LocaleSpanish: "Generar una respuesta"}, flow.PurityRead, apigen.NONE}},
	},
	{
		typ: "orcon.ai.agent@1", group: groupApps,
		name: map[string]string{LocaleEnglish: "AI agent", LocaleSpanish: "Agente de IA"},
		desc: map[string]string{
			LocaleEnglish: "Lets an AI model decide steps and call tools, with memory and a spending limit, until it has an answer.",
			LocaleSpanish: "Deja que un modelo de IA decida pasos y llame herramientas, con memoria y un límite de gasto, hasta tener una respuesta.",
		},
		operations: []builtinOperation{{"agent.run", map[string]string{LocaleEnglish: "Run the agent", LocaleSpanish: "Ejecutar el agente"}, flow.PurityAgentic, apigen.NONE}},
	},
	{
		typ: "orcon.if@1", group: groupFlow,
		name: map[string]string{LocaleEnglish: "IF", LocaleSpanish: "Condición (IF)"},
		desc: map[string]string{
			LocaleEnglish: "Sends each item down the true or the false branch by a condition.",
			LocaleSpanish: "Envía cada elemento por la rama verdadera o falsa según una condición.",
		},
		operations: []builtinOperation{{"condition.route", map[string]string{LocaleEnglish: "Route by condition", LocaleSpanish: "Enrutar por condición"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.switch@1", group: groupFlow,
		name: map[string]string{LocaleEnglish: "Switch", LocaleSpanish: "Selector (Switch)"},
		desc: map[string]string{
			LocaleEnglish: "Sends each item down one of several branches by rules.",
			LocaleSpanish: "Envía cada elemento por una de varias ramas según reglas.",
		},
		operations: []builtinOperation{{"rule.route", map[string]string{LocaleEnglish: "Route by rules", LocaleSpanish: "Enrutar por reglas"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.loop@1", group: groupFlow,
		name: map[string]string{LocaleEnglish: "Loop", LocaleSpanish: "Bucle"},
		desc: map[string]string{
			LocaleEnglish: "Repeats the steps inside it, a batch of items at a time.",
			LocaleSpanish: "Repite los pasos de su interior, un lote de elementos cada vez.",
		},
		operations: []builtinOperation{{"batch.iterate", map[string]string{LocaleEnglish: "Iterate over batches", LocaleSpanish: "Iterar por lotes"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.set@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Edit fields", LocaleSpanish: "Editar campos"},
		desc: map[string]string{
			LocaleEnglish: "Sets, renames or removes fields of every item.",
			LocaleSpanish: "Asigna, renombra o elimina campos de cada elemento.",
		},
		operations: []builtinOperation{{"field.assign", map[string]string{LocaleEnglish: "Assign fields", LocaleSpanish: "Asignar campos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.noop@1", group: groupData,
		name: map[string]string{LocaleEnglish: "No operation", LocaleSpanish: "Sin operación"},
		desc: map[string]string{
			LocaleEnglish: "Passes the items through unchanged.",
			LocaleSpanish: "Deja pasar los elementos sin cambios.",
		},
		operations: []builtinOperation{{"item.pass", map[string]string{LocaleEnglish: "Pass items through", LocaleSpanish: "Dejar pasar los elementos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.filter@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Filter", LocaleSpanish: "Filtrar"},
		desc: map[string]string{
			LocaleEnglish: "Keeps the items that meet a condition and drops the rest.",
			LocaleSpanish: "Conserva los elementos que cumplen una condición y descarta el resto.",
		},
		operations: []builtinOperation{{"item.filter", map[string]string{LocaleEnglish: "Keep items by condition", LocaleSpanish: "Conservar elementos por condición"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.sort@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Sort", LocaleSpanish: "Ordenar"},
		desc: map[string]string{
			LocaleEnglish: "Puts the items in order by one or more fields, or in a random order.",
			LocaleSpanish: "Pone los elementos en orden según uno o más campos, o en orden aleatorio.",
		},
		operations: []builtinOperation{{"item.sort", map[string]string{LocaleEnglish: "Sort items", LocaleSpanish: "Ordenar elementos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.limit@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Limit", LocaleSpanish: "Limitar"},
		desc: map[string]string{
			LocaleEnglish: "Keeps only the first or the last items, up to a number you choose.",
			LocaleSpanish: "Conserva solo los primeros o los últimos elementos, hasta un número que eliges.",
		},
		operations: []builtinOperation{{"item.limit", map[string]string{LocaleEnglish: "Limit the number of items", LocaleSpanish: "Limitar el número de elementos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.removeDuplicates@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Remove duplicates", LocaleSpanish: "Quitar duplicados"},
		desc: map[string]string{
			LocaleEnglish: "Drops the items that repeat an earlier item, comparing all fields or some of them.",
			LocaleSpanish: "Descarta los elementos que repiten uno anterior, comparando todos los campos o solo algunos.",
		},
		operations: []builtinOperation{{"item.deduplicate", map[string]string{LocaleEnglish: "Remove repeated items", LocaleSpanish: "Quitar elementos repetidos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.aggregate@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Aggregate", LocaleSpanish: "Agrupar elementos"},
		desc: map[string]string{
			LocaleEnglish: "Gathers the items, or the values of some of their fields, into lists in a single item.",
			LocaleSpanish: "Reúne los elementos, o los valores de algunos de sus campos, en listas dentro de un solo elemento.",
		},
		operations: []builtinOperation{{"item.aggregate", map[string]string{LocaleEnglish: "Gather items into one", LocaleSpanish: "Reunir los elementos en uno"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.splitOut@1", group: groupData,
		name: map[string]string{LocaleEnglish: "Split out", LocaleSpanish: "Separar lista"},
		desc: map[string]string{
			LocaleEnglish: "Turns each element of a list field into an item of its own.",
			LocaleSpanish: "Convierte cada elemento de un campo de lista en un elemento propio.",
		},
		operations: []builtinOperation{{"item.split", map[string]string{LocaleEnglish: "Split a list into items", LocaleSpanish: "Separar una lista en elementos"}, flow.PurityPure, apigen.NONE}},
	},
	{
		typ: "orcon.stopAndError@1", group: groupFlow,
		name: map[string]string{LocaleEnglish: "Stop and error", LocaleSpanish: "Detener con error"},
		desc: map[string]string{
			LocaleEnglish: "Ends the execution with an error and a code you choose.",
			LocaleSpanish: "Termina la ejecución con un error y un código que tú eliges.",
		},
		operations: []builtinOperation{{"execution.fail", map[string]string{LocaleEnglish: "Fail the execution", LocaleSpanish: "Hacer fallar la ejecución"}, flow.PurityPure, apigen.NONE}},
	},
}

// builtinNodeTypes describes the built-in node types in the language.
func builtinNodeTypes(locale string) []apigen.NodeType {
	out := make([]apigen.NodeType, 0, len(builtins))
	for i := range builtins {
		b := &builtins[i]
		nt := apigen.NodeType{
			Type: b.typ, Name: b.name[locale], Description: b.desc[locale], Group: b.group,
			Seal: apigen.NodeTypeSealOFFICIAL, ProofLevel: apigen.NodeTypeProofLevel(connector.ProofVerified),
			Operations: make([]apigen.NodeTypeOperation, 0, len(b.operations)), N8nTypes: n8nTypesOf(b.typ),
		}
		for _, op := range b.operations {
			nt.Operations = append(nt.Operations, apigen.NodeTypeOperation{
				Name: op.name, Label: op.label[locale], Purity: apigen.NodeTypeOperationPurity(op.purity), SafetyLevel: op.safety,
			})
		}
		out = append(out, nt)
	}
	return out
}

// n8nTypesOf lists the n8n node types the importer maps to an Orcon type (SPEC-003 Annex B).
func n8nTypesOf(orconType string) []string {
	out := []string{}
	for _, row := range n8nimport.CoreNodes() {
		for _, t := range row.N8nTypes {
			if row.Orcon == orconType || (row.Orcon == n8nimport.NamedTemplate && "orcon."+t+"@1" == orconType) {
				out = append(out, n8nPrefix+t)
			}
		}
	}
	return out
}
