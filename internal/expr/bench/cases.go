// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Case is a benchmark expression with the data it runs against. The set in
// [Cases] is written for this repository from the idioms that dominate the
// public template corpus (member access, string and array methods, date
// arithmetic, references to other nodes); none of it is taken from a
// template.
type Case struct {
	Name string
	// Template is the parameter value, with its leading "=".
	Template string
	// Item is the JSON of $json.
	Item string
	// Nodes maps node names to the JSON array of their output items.
	Nodes map[string]string
	// Luxon marks the cases that run Luxon, the largest JavaScript library of
	// the runtime and the slowest to interpret.
	Luxon bool
}

const sampleItem = `{"id":1042,"name":"Ada Lovelace","first":"Ada","last":"Lovelace","email":"Ada.Lovelace@Example.test",
"count":3,"price":19.99,"qty":4,"total":250,"amount":1234.5,"currency":"EUR","a":0,"b":"yes","text":"  the quick   brown fox  jumps ",
"date":"2026-03-14T10:30:00Z","ts":1773484200000,"tags":["red","green","blue"],"list":[{"name":"first"},{"name":"second"}],
"user":{"address":{"city":"London","zip":"N1"}},"obj":{"x":1,"y":2,"z":3},
"items":[{"name":"pen","price":2.5,"qty":3},{"name":"book","price":12,"qty":1},{"name":"lamp","price":30,"qty":2}],
"people":[{"name":"a","age":20},{"name":"b","age":35},{"name":"c","age":50}]}`

// Cases returns the benchmark expressions.
func Cases() []Case {
	nodes := map[string]string{"Fetch": `[{"json":{"id":7,"token":"t-1"}},{"json":{"id":8,"token":"t-2"}}]`}
	mk := func(name, tmpl string) Case { return Case{Name: name, Template: tmpl, Item: sampleItem, Nodes: nodes} }
	luxon := func(name, tmpl string) Case {
		c := mk(name, tmpl)
		c.Luxon = true
		return c
	}
	return []Case{
		mk("member", `={{ $json.name }}`),
		mk("deep member", `={{ $json.user.address.city }}`),
		mk("other node", `={{ $('Fetch').item.json.id }}`),
		mk("arithmetic", `={{ $json.price * $json.qty }}`),
		mk("conditional", `={{ $json.total > 100 ? 'large' : 'small' }}`),
		mk("text template", `=Hello {{ $json.first }} {{ $json.last }}, you have {{ $json.count }} items`),
		mk("first input", `={{ $input.first().json.id }}`),
		mk("logical", `={{ $json.a || $json.b || 'fallback' }}`),
		mk("list index", `={{ $json.list.length > 0 ? $json.list[0].name : 'none' }}`),
		mk("string method", `={{ $json.name.toUpperCase() }}`),
		mk("array join", `={{ $json.tags.join(', ') }}`),
		mk("map and reduce", `={{ $json.items.map(i => i.price * i.qty).reduce((a, b) => a + b, 0) }}`),
		mk("filter and length", `={{ $json.items.filter(i => i.qty > 1).length }}`),
		luxon("date format", `={{ $now.toFormat('yyyy-MM-dd') }}`),
		luxon("date arithmetic", `={{ $now.plus({ days: 7 }).toISO() }}`),
		luxon("date parse", `={{ DateTime.fromISO($json.date).toFormat('dd/MM/yyyy') }}`),
		mk("json stringify", `={{ JSON.stringify($json.obj) }}`),
		mk("text cleanup", `={{ $json.text.replace(/\s+/g, ' ').trim().split(' ').length }}`),
		mk("email check", `={{ $json.email.toLowerCase().includes('@') }}`),
		mk("object keys", `={{ Object.keys($json.obj).length }}`),
		mk("extension pluck", `={{ $json.items.pluck('name').join('|') }}`),
		mk("extension title case", `={{ $json.name.toTitleCase() }}`),
		mk("all items of a node", `={{ $('Fetch').all().map(i => i.json.id).join(',') }}`),
		mk("sort", `={{ [$json.price, $json.qty, $json.count].sort((x, y) => x - y)[0] }}`),
		mk("jmespath", "={{ $jmespath($json.people, '[?age > `30`].name') }}"),
		mk("number format", `={{ $json.amount.toFixed(2) + ' ' + $json.currency }}`),
		mk("date from timestamp", `={{ new Date($json.ts).toISOString() }}`),
		mk("string pipeline", `={{ $json.email.split('@')[0].replace('.', ' ').toTitleCase() }}`),
	}
}

// Env builds the evaluation context of the case with a fixed activation.
func (c Case) Env() (*expr.Env, error) {
	item, err := jsonvalue.Parse([]byte(c.Item))
	if err != nil {
		return nil, fmt.Errorf("bench: case %q item: %w", c.Name, err)
	}
	nodes := fixedNodes{}
	for name, text := range c.Nodes {
		arr, err := jsonvalue.Parse([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("bench: case %q node %q: %w", c.Name, name, err)
		}
		var items []expr.Item
		for _, it := range arr.Items() {
			j, _ := it.Get("json")
			items = append(items, expr.Item{JSON: j})
		}
		nodes[name] = items
	}
	var seed [expr.SeedSize]byte
	return &expr.Env{
		Input:      []expr.Item{{JSON: item}},
		Nodes:      nodes,
		Activation: expr.NewActivation(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), seed),
		Timezone:   "UTC",
	}, nil
}

// fixedNodes is an [expr.NodeSource] over fixed output items; the item paired
// with item i is item i.
type fixedNodes map[string][]expr.Item

func (f fixedNodes) Node(name string) (expr.NodeOutput, bool) {
	items, ok := f[name]
	if !ok {
		return expr.NodeOutput{}, false
	}
	return expr.NodeOutput{
		Executed: true,
		Items:    items,
		Params:   jsonvalue.Null(),
		Paired: func(i int) (expr.Item, bool) {
			if i < 0 || i >= len(items) {
				return expr.Item{}, false
			}
			return items[i], true
		},
	}, true
}
