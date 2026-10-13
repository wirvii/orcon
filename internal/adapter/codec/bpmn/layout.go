// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import "github.com/wirvii/orcon/internal/flow"

// The editor member of a flow is opaque to the format, so the BPMNDI diagram
// is not taken from it: it is a deterministic layered layout of the graph,
// enough for other BPMN tools to draw the flow. Import ignores it.

type layoutNode struct {
	id      string
	element flow.Element
}

type layoutEdge struct {
	id, from, to string
	back         bool
}

type bounds struct{ x, y, w, h int }

type point struct{ x, y int }

type diagram struct {
	nodes map[string]bounds
	edges map[string][2]point
}

const (
	colWidth  = 180
	rowHeight = 120
	margin    = 40
)

func sizeOf(e flow.Element) (w, h int) {
	switch e {
	case flow.ElementStartEvent, flow.ElementEndEvent, flow.ElementIntermediateCatchEvent,
		flow.ElementIntermediateThrowEvent, flow.ElementBoundaryEvent:
		return 36, 36
	case flow.ElementExclusiveGateway, flow.ElementInclusiveGateway, flow.ElementParallelGateway,
		flow.ElementEventBasedGateway, flow.ElementComplexGateway:
		return 50, 50
	case flow.ElementTask, flow.ElementServiceTask, flow.ElementScriptTask, flow.ElementUserTask,
		flow.ElementReceiveTask, flow.ElementSendTask, flow.ElementCallActivity:
		return 100, 80
	default:
		return 100, 80
	}
}

// layoutGraph places each node in the column of its longest forward path and
// in the next free row of that column.
func layoutGraph(nodes []layoutNode, edges []layoutEdge) diagram {
	layer := make(map[string]int, len(nodes))
	for range nodes {
		changed := false
		for _, e := range edges {
			if e.back {
				continue
			}
			if layer[e.to] < layer[e.from]+1 {
				layer[e.to] = layer[e.from] + 1
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	rows := make(map[int]int)
	d := diagram{nodes: make(map[string]bounds, len(nodes)), edges: make(map[string][2]point, len(edges))}
	centers := make(map[string]point, len(nodes))
	sizes := make(map[string][2]int, len(nodes))
	for _, n := range nodes {
		col := layer[n.id]
		row := rows[col]
		rows[col]++
		w, h := sizeOf(n.element)
		cx, cy := margin+col*colWidth, margin+row*rowHeight
		d.nodes[n.id] = bounds{x: cx - w/2, y: cy - h/2, w: w, h: h}
		centers[n.id] = point{cx, cy}
		sizes[n.id] = [2]int{w, h}
	}
	for _, e := range edges {
		from, to := centers[e.from], centers[e.to]
		d.edges[e.id] = [2]point{
			{from.x + sizes[e.from][0]/2, from.y},
			{to.x - sizes[e.to][0]/2, to.y},
		}
	}
	return d
}
