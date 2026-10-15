// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authoring

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
)

// MaxIconBytes is the size limit of one SVG document of an icon (IC-3: at most 8 KiB).
const MaxIconBytes = 8 << 10

const darkIconFile = "icons/icon-dark.svg"

// ErrUnsafeIcon means an SVG document carries something an icon must not: a script,
// an event attribute, an external reference, a markup declaration or a foreign object.
var ErrUnsafeIcon = errors.New("unsafe SVG icon")

// forbiddenElements are elements an icon never uses: they run code, load other
// documents, or change the document after it is drawn.
var forbiddenElements = map[string]bool{
	"script": true, "foreignobject": true, "iframe": true, "object": true, "embed": true,
	"style": true, "image": true, "a": true, "animate": true, "animatemotion": true,
	"animatetransform": true, "animatecolor": true, "set": true, "handler": true, "listener": true,
}

// SanitizeSVG checks that markup is an icon that is safe to show inline: one svg root,
// at most MaxIconBytes, no forbidden element, no attribute that starts with "on", no
// reference that leaves the document (href and url() must point inside it), and no
// markup declaration (DOCTYPE or entities). It returns the markup unchanged when it passes.
func SanitizeSVG(markup []byte) ([]byte, error) {
	if len(markup) > MaxIconBytes {
		return nil, fmt.Errorf("%w: %d bytes, the limit is %d", ErrUnsafeIcon, len(markup), MaxIconBytes)
	}
	dec := xml.NewDecoder(bytes.NewReader(markup))
	dec.Strict = true
	root := true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: not well-formed XML: %v", ErrUnsafeIcon, err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("%w: markup declarations (DOCTYPE, entities) are not allowed", ErrUnsafeIcon)
		case xml.ProcInst:
			if t.Target != "xml" {
				return nil, fmt.Errorf("%w: processing instruction %q is not allowed", ErrUnsafeIcon, t.Target)
			}
		case xml.StartElement:
			if root {
				if !strings.EqualFold(t.Name.Local, "svg") {
					return nil, fmt.Errorf("%w: the root element is %q, want svg", ErrUnsafeIcon, t.Name.Local)
				}
				root = false
			}
			if err := checkElement(t); err != nil {
				return nil, err
			}
		}
	}
	if root {
		return nil, fmt.Errorf("%w: no svg root element", ErrUnsafeIcon)
	}
	return markup, nil
}

func checkElement(el xml.StartElement) error {
	name := strings.ToLower(el.Name.Local)
	if forbiddenElements[name] {
		return fmt.Errorf("%w: element %q is not allowed in an icon", ErrUnsafeIcon, el.Name.Local)
	}
	for _, attr := range el.Attr {
		attrName := strings.ToLower(attr.Name.Local)
		value := strings.TrimSpace(attr.Value)
		switch {
		case strings.HasPrefix(attrName, "on"):
			return fmt.Errorf("%w: event attribute %q is not allowed", ErrUnsafeIcon, attr.Name.Local)
		case attrName == "href":
			if !strings.HasPrefix(value, "#") {
				return fmt.Errorf("%w: %s on %q must point inside the icon (#id)", ErrUnsafeIcon, attr.Name.Local, el.Name.Local)
			}
		case strings.Contains(strings.ToLower(value), "url("):
			if !strings.Contains(strings.ToLower(value), "url(#") {
				return fmt.Errorf("%w: %s on %q references something outside the icon", ErrUnsafeIcon, attr.Name.Local, el.Name.Local)
			}
		}
		if strings.Contains(strings.ToLower(value), "javascript:") || strings.Contains(strings.ToLower(value), "data:") {
			return fmt.Errorf("%w: %s on %q carries a script or data URL", ErrUnsafeIcon, attr.Name.Local, el.Name.Local)
		}
	}
	return nil
}

// Icon is the brand icon of a connector package, checked and ready to serve inline.
type Icon struct {
	// SVG is the icon in the brand's colour.
	SVG string
	// DarkSVG is the variant the brand publishes for dark backgrounds; empty when the brand
	// publishes none (orcon-connector IC-7).
	DarkSVG string
	// Source is the single line of icons/SOURCE (orcon-connector IC-6).
	Source string
}

// LoadIcon reads and checks the icon of the connector package whose root is fsys: the
// mark, the optional brand dark variant and the source line. It is the step that runs when
// a connector is loaded, so an unsafe icon never reaches the API.
func LoadIcon(fsys fs.FS) (Icon, error) {
	svg, err := fs.ReadFile(fsys, iconFile)
	if err != nil {
		return Icon{}, fmt.Errorf("read %s: %w", iconFile, err)
	}
	if _, err := SanitizeSVG(svg); err != nil {
		return Icon{}, fmt.Errorf("%s: %w", iconFile, err)
	}
	icon := Icon{SVG: string(svg)}
	if dark, err := fs.ReadFile(fsys, darkIconFile); err == nil {
		if _, err := SanitizeSVG(dark); err != nil {
			return Icon{}, fmt.Errorf("%s: %w", darkIconFile, err)
		}
		icon.DarkSVG = string(dark)
	}
	source, err := fs.ReadFile(fsys, sourceFile)
	if err != nil {
		return Icon{}, fmt.Errorf("read %s: %w", sourceFile, err)
	}
	line := strings.TrimSpace(string(source))
	if !sourceLine.MatchString(line) {
		return Icon{}, fmt.Errorf("%s is not one line of simple-icons@<version>:<slug>, brand-kit:<url> or orcon-fallback", sourceFile)
	}
	icon.Source = line
	return icon, nil
}
