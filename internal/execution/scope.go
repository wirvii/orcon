// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
)

// ScopePath identifies the loop instance and iteration and the multi-instance
// instance a token runs in (ADR-0010 section 1, amendment 9). Entering a loop
// appends "L:<loopNodeId>@<entryTokenId>#<i>", where the entry token is the
// live token that arrived on the loop's entry edge (every such token runs its
// own instance) and i is the iteration; a multi-instance instance appends
// "MI:<nodeId>#<k>". Frames are separated by "/". Node ids never contain "/",
// "@" or "#" (ADR-0007 section 3) and token ids are canonical UUID text, so the
// text is unambiguous. The empty path is the top level.
//
// Joins, merge trackers and data reads match only within one scope path. A
// ScopePath is comparable and is built only with [ScopePath.Push].
type ScopePath string

// Scope frame kinds.
const (
	// ScopeLoop marks a loop iteration frame.
	ScopeLoop = "L"
	// ScopeMultiInstance marks a multi-instance instance frame.
	ScopeMultiInstance = "MI"
)

const scopeSeparator = "/"

// ScopeFrame is one level of a [ScopePath].
type ScopeFrame struct {
	// Kind is [ScopeLoop] or [ScopeMultiInstance].
	Kind string
	// Node is the loop control or multi-instance node that opened the frame.
	Node flow.NodeID
	// Entry is the token that entered a loop; it is empty for multi-instance
	// frames.
	Entry TokenID
	// Index is the iteration or instance number, starting at 0.
	Index int
}

// LoopFrame returns the frame of iteration i of the loop instance a token
// started on a loop control node.
func LoopFrame(node flow.NodeID, entry TokenID, i int) ScopeFrame {
	return ScopeFrame{Kind: ScopeLoop, Node: node, Entry: entry, Index: i}
}

// MultiInstanceFrame returns the frame of instance k of a multi-instance node.
func MultiInstanceFrame(node flow.NodeID, k int) ScopeFrame {
	return ScopeFrame{Kind: ScopeMultiInstance, Node: node, Index: k}
}

// String renders the frame as it appears in a scope path.
func (f ScopeFrame) String() string {
	if f.Kind == ScopeLoop {
		return f.Kind + ":" + string(f.Node) + "@" + string(f.Entry) + "#" + strconv.Itoa(f.Index)
	}
	return f.Kind + ":" + string(f.Node) + "#" + strconv.Itoa(f.Index)
}

// Push returns the path with one more frame.
func (s ScopePath) Push(f ScopeFrame) ScopePath {
	if s == "" {
		return ScopePath(f.String())
	}
	return s + scopeSeparator + ScopePath(f.String())
}

// Pop returns the path without its innermost frame and that frame. It reports
// false on the top-level path.
func (s ScopePath) Pop() (ScopePath, ScopeFrame, bool) {
	if s == "" {
		return "", ScopeFrame{}, false
	}
	text := string(s)
	head, last := "", text
	if cut := strings.LastIndex(text, scopeSeparator); cut >= 0 {
		head, last = text[:cut], text[cut+1:]
	}
	frame, err := parseFrame(last)
	if err != nil {
		return "", ScopeFrame{}, false
	}
	return ScopePath(head), frame, true
}

// PopN pops n frames and reports false when the path is shallower than n.
func (s ScopePath) PopN(n int) (ScopePath, bool) {
	for range n {
		next, _, ok := s.Pop()
		if !ok {
			return s, false
		}
		s = next
	}
	return s, true
}

// Depth returns the number of frames.
func (s ScopePath) Depth() int {
	if s == "" {
		return 0
	}
	return strings.Count(string(s), scopeSeparator) + 1
}

// Frames returns the frames from the outermost to the innermost. A path that
// was not built by [ScopePath.Push] or [ParseScopePath] yields nil.
func (s ScopePath) Frames() []ScopeFrame {
	frames, err := parseFrames(string(s))
	if err != nil {
		return nil
	}
	return frames
}

// ParseScopePath validates the text form of a scope path, for example one read
// back from storage.
func ParseScopePath(text string) (ScopePath, error) {
	if _, err := parseFrames(text); err != nil {
		return "", err
	}
	return ScopePath(text), nil
}

func parseFrames(text string) ([]ScopeFrame, error) {
	if text == "" {
		return nil, nil
	}
	parts := strings.Split(text, scopeSeparator)
	frames := make([]ScopeFrame, 0, len(parts))
	for _, part := range parts {
		f, err := parseFrame(part)
		if err != nil {
			return nil, err
		}
		frames = append(frames, f)
	}
	return frames, nil
}

func parseFrame(text string) (ScopeFrame, error) {
	kind, rest, ok := strings.Cut(text, ":")
	if !ok || (kind != ScopeLoop && kind != ScopeMultiInstance) {
		return ScopeFrame{}, fmt.Errorf("scope frame %q: kind must be %s or %s", text, ScopeLoop, ScopeMultiInstance)
	}
	head, index, ok := strings.Cut(rest, "#")
	if !ok || head == "" {
		return ScopeFrame{}, fmt.Errorf("scope frame %q: want <kind>:<node>[@<entry>]#<index>", text)
	}
	node, entry, hasEntry := strings.Cut(head, "@")
	switch {
	case node == "":
		return ScopeFrame{}, fmt.Errorf("scope frame %q: no node", text)
	case kind == ScopeLoop && (!hasEntry || !canonicalID(entry)):
		return ScopeFrame{}, fmt.Errorf("scope frame %q: a loop frame names its entry token as canonical id text after @", text)
	case kind == ScopeMultiInstance && hasEntry:
		return ScopeFrame{}, fmt.Errorf("scope frame %q: a multi-instance frame has no entry token", text)
	}
	n, err := strconv.Atoi(index)
	// The index is written in its canonical form, so a path has one text.
	if err != nil || n < 0 || strconv.Itoa(n) != index {
		return ScopeFrame{}, fmt.Errorf("scope frame %q: index must be a non-negative integer without leading zeros or sign", text)
	}
	return ScopeFrame{Kind: kind, Node: flow.NodeID(node), Entry: TokenID(entry), Index: n}, nil
}

// canonicalID reports whether text is the canonical form of a deterministic id:
// 36 characters of lowercase hexadecimal digits in the 8-4-4-4-12 grouping.
func canonicalID(text string) bool {
	if len(text) != 36 {
		return false
	}
	for i := range len(text) {
		c := text[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}
