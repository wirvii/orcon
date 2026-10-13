// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// StopReason says why the selection ended.
type StopReason string

// Stop reasons (UPPER_SNAKE: they appear in the report).
const (
	// StopTarget: the target percentage was reached.
	StopTarget StopReason = "TARGET_REACHED"
	// StopNoProgress: no connector brings any template closer; what remains
	// needs something other than a connector.
	StopNoProgress StopReason = "NO_PROGRESS"
	// StopLimit: the connector limit was reached first.
	StopLimit StopReason = "LIMIT_REACHED"
)

// DefaultTarget is the release gate of ADR-0037 section 4, in percent.
const DefaultTarget = 90.0

// SelectOptions tune [SelectWave]. The zero value selects until the release
// target with the default effort estimate and no connector limit.
type SelectOptions struct {
	// Target is the percentage of templates to reach; 0 means [DefaultTarget].
	Target float64
	// EffortOf estimates the cost class of the connector for an n8n type; nil
	// means [DefaultEffort].
	EffortOf func(nodeType string) Effort
	// MaxConnectors stops the selection after that many connectors; 0 means
	// no limit.
	MaxConnectors int
}

// Step is one connector chosen by the greedy selection.
type Step struct {
	Rank   int
	Type   string
	Effort Effort
	// Units are the operations the connector has to declare: every unit of
	// the type that a still-reachable template uses.
	Units int
	// Newly is the number of templates that become fully importable with this
	// connector (AI nodes not counted).
	Newly int
	// Templates and Occurrences measure how much the corpus uses the type.
	Templates   int
	Occurrences int
	// Importable and Percent are the totals after this step.
	Importable int
	Percent    float64
	// CumulativeEffort is the sum of the weights up to this step.
	CumulativeEffort int
}

// Selection is the wave-1 plan.
type Selection struct {
	Target    float64
	Templates int
	// Start is the number of templates importable before any connector.
	Start int
	// Ceiling is the number of templates no connector alone can leave
	// blocked: those without a blocker a connector cannot fix.
	Ceiling int
	Steps   []Step
	Stop    StopReason
	// Catalog is the base catalog plus the selected connectors, ready to
	// measure or write out.
	Catalog *Catalog
}

// nativeTypes are the built-in types whose protocol the declarative HTTP
// format cannot speak (databases, queues, mail transports); every other
// integration is assumed declarative. It is an estimate for ordering, not a
// design decision: the connector author decides (ADR-0079 section 9).
var nativeTypes = map[string]bool{
	"postgres": true, "mySql": true, "microsoftSql": true, "mongoDb": true, "redis": true,
	"snowflake": true, "oracleDatabase": true, "questDb": true, "timescaleDb": true, "crateDb": true,
	"kafka": true, "rabbitmq": true, "amqp": true, "mqtt": true, "emailReadImap": true,
	"postgresTrigger": true, "redisTrigger": true, "kafkaTrigger": true, "rabbitmqTrigger": true,
	"amqpTrigger": true, "mqttTrigger": true,
}

// DefaultEffort estimates the cost class of a connector for a built-in n8n
// type: native for protocols and databases, declarative otherwise.
func DefaultEffort(nodeType string) Effort {
	if nativeTypes[strings.TrimPrefix(nodeType, basePrefix)] {
		return EffortNative
	}
	return EffortDeclarative
}

// fracOne is the fixed-point value of 1 for the progress credit; integer
// arithmetic keeps the selection identical on every platform.
const fracOne = int64(1) << 40

type candidate struct {
	typ         string
	units       map[Unit]bool
	templates   int
	occurrences int
}

type live struct {
	res  *Result
	need map[string]bool
}

// SelectWave chooses connectors greedily (ADR-0037 section 4): at each step
// it adds the connector that makes the most templates fully importable per
// unit of effort. When no single connector completes a template, ties go to
// the one that brings the most templates closer (each template gives 1/k to
// each of the k connectors it still lacks), then to the most used type, then
// to the type name. AI nodes count as the catalog maps them; the planned base
// catalog maps them as SPEC-006 Annex B commits.
func SelectWave(c *Corpus, base *Catalog, opts SelectOptions) (*Selection, error) {
	if opts.Target == 0 {
		opts.Target = DefaultTarget
	}
	effortOf := opts.EffortOf
	if effortOf == nil {
		effortOf = DefaultEffort
	}
	results := make([]Result, len(c.Templates))
	for i := range c.Templates {
		results[i] = Classify(&c.Templates[i], base)
	}
	sel := &Selection{Target: opts.Target, Templates: len(results)}
	var open []live
	cands := map[string]*candidate{}
	for i := range results {
		r := &results[i]
		if r.Importable() {
			sel.Start++
			sel.Ceiling++
			continue
		}
		if r.Fixed() {
			continue
		}
		sel.Ceiling++
		need := map[string]bool{}
		for _, b := range r.Blockers {
			need[b.Unit.Type] = true
			cd := cands[b.Unit.Type]
			if cd == nil {
				cd = &candidate{typ: b.Unit.Type, units: map[Unit]bool{}}
				cands[b.Unit.Type] = cd
			}
			cd.units[b.Unit] = true
		}
		for typ := range need {
			cands[typ].templates++
		}
		open = append(open, live{res: r, need: need})
	}
	for i := range c.Templates {
		for j := range c.Templates[i].Nodes {
			typ, _ := normalizeType(c.Templates[i].Nodes[j].Type)
			if cd := cands[typ]; cd != nil {
				cd.occurrences++
			}
		}
	}

	selected := map[string]bool{}
	importable := sel.Start
	effort := 0
	catalog := base
	for percent(importable, sel.Templates) < opts.Target {
		if opts.MaxConnectors > 0 && len(sel.Steps) >= opts.MaxConnectors {
			sel.Stop = StopLimit
			break
		}
		best, gain := pickBest(cands, open, selected, effortOf)
		if best == nil {
			sel.Stop = StopNoProgress
			break
		}
		selected[best.typ] = true
		importable += gain
		eff := effortOf(best.typ)
		effort += eff.Weight()
		sel.Steps = append(sel.Steps, Step{
			Rank: len(sel.Steps) + 1, Type: best.typ, Effort: eff, Units: len(best.units), Newly: gain,
			Templates: best.templates, Occurrences: best.occurrences,
			Importable: importable, Percent: percent(importable, sel.Templates), CumulativeEffort: effort,
		})
		var err error
		catalog, err = withConnector(catalog, best, eff)
		if err != nil {
			return nil, err
		}
	}
	if sel.Stop == "" {
		sel.Stop = StopTarget
	}
	sel.Catalog = catalog
	return sel, nil
}

func percent(n, d int) float64 { return Pct(int64(n), int64(d)) }

// pickBest returns the unselected candidate with the best score and the
// number of templates it completes, or nil when none makes progress.
func pickBest(cands map[string]*candidate, open []live, selected map[string]bool, effortOf func(string) Effort) (*candidate, int) {
	type score struct {
		gain int
		frac int64
	}
	scores := map[string]*score{}
	for _, l := range open {
		var missing []string
		for typ := range l.need {
			if !selected[typ] {
				missing = append(missing, typ)
			}
		}
		if len(missing) == 0 {
			continue
		}
		for _, typ := range missing {
			s := scores[typ]
			if s == nil {
				s = &score{}
				scores[typ] = s
			}
			s.frac += fracOne / int64(len(missing))
			if len(missing) == 1 {
				s.gain++
			}
		}
	}
	var best *candidate
	var bestScore *score
	for typ, s := range scores {
		cd := cands[typ]
		w := int64(effortOf(typ).Weight())
		if best == nil {
			best, bestScore = cd, s
			continue
		}
		bw := int64(effortOf(best.typ).Weight())
		c := cmp.Or(
			cmp.Compare(int64(s.gain)*bw, int64(bestScore.gain)*w),
			cmp.Compare(s.frac*bw, bestScore.frac*w),
			cmp.Compare(cd.templates, best.templates),
			cmp.Compare(best.typ, cd.typ), // the lower name wins a full tie
		)
		if c > 0 {
			best, bestScore = cd, s
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, bestScore.gain
}

// withConnector returns the catalog plus a connector for the candidate.
func withConnector(cat *Catalog, cd *candidate, eff Effort) (*Catalog, error) {
	units := make([]UnitMap, 0, len(cd.units))
	for u := range cd.units {
		units = append(units, UnitMap{Unit: u})
	}
	slices.SortFunc(units, func(a, b UnitMap) int { return strings.Compare(a.String(), b.String()) })
	id := strings.TrimPrefix(cd.typ, basePrefix)
	var all []Connector
	if cat != nil {
		all = append(all, cat.Connectors...)
	}
	all = append(all, Connector{ID: id, Effort: eff, Units: units})
	next, err := NewCatalog(all...)
	if err != nil {
		return nil, fmt.Errorf("n8nimport: add the connector for %s: %w", cd.typ, err)
	}
	if cat != nil {
		next = next.WithAI(cat.AI...)
	}
	return next, nil
}
