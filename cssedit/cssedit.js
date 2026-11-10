/*
 * Copyright © 2026 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

// CodeMirror 6 editor for the Custom CSS field on the blog Customize page.
// Built into static/js/cssedit.bundle.js, which exposes the WFCSSEditor global.

import {EditorView, keymap, drawSelection, highlightSpecialChars} from "@codemirror/view";
import {defaultKeymap, history, historyKeymap, indentWithTab, deleteLine} from "@codemirror/commands";
import {syntaxHighlighting, defaultHighlightStyle, bracketMatching, indentOnInput} from "@codemirror/language";
import {css} from "@codemirror/lang-css";

// Accessibility attributes moved from the textarea onto the editor's content
// element, so assistive tech announces the editor the same way.
var ariaAttrs = ["aria-label", "aria-labelledby", "aria-describedby"];

// fromTextArea replaces the given <textarea> with a CSS editor. The textarea
// stays in its form (hidden) and is kept in sync on every change, so it's
// still the field that gets submitted.
//
// Tab indents (Shift+Tab outdents). Keyboard users leave the editor by pressing
// Escape and then Tab, or by toggling Tab's behavior with Ctrl+M
// (Shift+Option+M on macOS); both come with CodeMirror's default keymap.
//
// Ctrl+D (Cmd+D on macOS) deletes the current line, as it did in the Ace
// editor this replaced. Shift+Ctrl+K (Shift+Cmd+K) does the same.
//
// Options:
//   describedBy: id of an element that describes the editor (e.g. its
//                keyboard instructions), added to aria-describedby.
export function fromTextArea(textarea, opts) {
	opts = opts || {};

	var contentAttrs = {};
	ariaAttrs.forEach(function(name) {
		if (textarea.hasAttribute(name)) {
			contentAttrs[name] = textarea.getAttribute(name);
		}
	});
	if (opts.describedBy) {
		contentAttrs["aria-describedby"] = contentAttrs["aria-describedby"]
			? contentAttrs["aria-describedby"] + " " + opts.describedBy
			: opts.describedBy;
	}

	var view = new EditorView({
		doc: textarea.value,
		extensions: [
			highlightSpecialChars(),
			history(),
			drawSelection(),
			indentOnInput(),
			syntaxHighlighting(defaultHighlightStyle, {fallback: true}),
			bracketMatching(),
			keymap.of([{key: "Mod-d", run: deleteLine, preventDefault: true}].concat(defaultKeymap, historyKeymap, [indentWithTab])),
			css(),
			EditorView.lineWrapping,
			EditorView.contentAttributes.of(contentAttrs),
			EditorView.updateListener.of(function(update) {
				if (update.docChanged) {
					textarea.value = update.state.doc.toString();
				}
			}),
		],
	});

	textarea.parentNode.insertBefore(view.dom, textarea.nextSibling);
	textarea.style.display = "none";
	return view;
}

// isMac reports whether the platform's Tab-toggle shortcut is Shift+Option+M
// instead of Ctrl+M, matching CodeMirror's own keymap logic.
export var isMac = typeof navigator != "undefined" && /Mac/.test(navigator.platform);
