// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

// maxUseDepth is how many <use> hops one chain of references may take.
const maxUseDepth = 3

// refEdge is a reference from one id'd element (or the document, "") to another.
// Containment counts as a reference: an element that holds an id'd child renders
// it. A <use> hop is the only edge that adds to the nesting depth.
type refEdge struct {
	to     string
	use    bool
	offset int64
}

// addRef records that the element owning the current position refers to id.
func (c *svgChecker) addRef(from, to string, use bool, offset int64) {
	if c.refs == nil {
		c.refs = map[string][]refEdge{}
	}
	c.refs[from] = append(c.refs[from], refEdge{to: to, use: use, offset: offset})
}

// checkReferences rejects reference cycles (use, gradient href, clip-path, mask
// and containment loops) and use chains deeper than maxUseDepth. A cycle makes
// a renderer loop or expand without bound; both are ways to hide or exhaust.
func (c *svgChecker) checkReferences() {
	state := map[string]int{} // 0 new, 1 on the stack, 2 done
	depth := map[string]int{}
	cycle := false
	var visit func(n string) int
	visit = func(n string) int {
		state[n] = 1
		best := 0
		for _, e := range c.refs[n] {
			d := 0
			switch state[e.to] {
			case 1:
				if !cycle {
					c.add(e.offset, "reference cycle: #%s refers back to #%s", n, e.to)
				}
				cycle = true
				continue
			case 0:
				d = visit(e.to)
			default:
				d = depth[e.to]
			}
			if e.use {
				d++
			}
			best = max(best, d)
		}
		state[n] = 2
		depth[n] = best
		return best
	}
	deepest := visit("")
	if !cycle && deepest > maxUseDepth {
		c.add(0, "use elements are nested %d deep, over the limit of %d", deepest, maxUseDepth)
	}
}
