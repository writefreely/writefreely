# cssedit

The code editor for the **Custom CSS** field on a blog's Customize page, built on [CodeMirror 6](https://codemirror.net).

`make ui` (from the repository root) builds it into `static/js/cssedit.bundle.js`. To build it on its own:

```bash
cd cssedit
npm ci
npm run build
```

The bundle exposes a `WFCSSEditor` global. `WFCSSEditor.fromTextArea(textarea, {describedBy: "id"})` replaces a `<textarea>` with the editor, keeping the hidden textarea in sync so the form submits as before. If the bundle isn't built or fails to load, the page falls back to the plain textarea.

Keyboard: Ctrl+D (Cmd+D on macOS) deletes the current line, as in the Ace editor this replaced. Tab indents and Shift+Tab outdents. To leave the editor, press Escape and then Tab, or press Ctrl+M (Shift+Option+M on macOS) to switch Tab to moving between fields.
