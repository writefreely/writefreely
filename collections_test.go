/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"strings"
	"testing"
)

// The description goes out as the ActivityPub actor's summary (PersonObject), which
// ActivityStreams defines as HTML, so its Markdown must be rendered.
func TestDisplayDescriptionRendersMarkdown(t *testing.T) {
	c := &Collection{Description: "Founder, [Musing Studio](https://musing.studio) / [Write.as](https://write.as)."}
	desc := string(*c.DisplayDescription())
	for _, want := range []string{`<a href="https://musing.studio"`, `>Musing Studio</a>`, `<a href="https://write.as"`} {
		if !strings.Contains(desc, want) {
			t.Errorf("wanted %q in %q", want, desc)
		}
	}
	if strings.Contains(desc, "](") {
		t.Errorf("Markdown left in %q", desc)
	}
}

func TestDisplayDescriptionEmpty(t *testing.T) {
	c := &Collection{}
	if desc := string(*c.DisplayDescription()); desc != "" {
		t.Errorf("wanted an empty description, got %q", desc)
	}
}
