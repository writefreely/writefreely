// class MarkdownView {
//   constructor(target, content) {
//     this.textarea = target.appendChild(document.createElement("textarea"))
//     this.textarea.value = content
//   }

//   get content() { return this.textarea.value }
//   focus() { this.textarea.focus() }
//   destroy() { this.textarea.remove() }
// }

import { EditorView } from "prosemirror-view";
import { EditorState, TextSelection } from "prosemirror-state";
import { exampleSetup } from "prosemirror-example-setup";
import { keymap } from "prosemirror-keymap";

import { writeFreelyMarkdownParser } from "./markdownParser";
import { writeFreelyMarkdownSerializer } from "./markdownSerializer";
import { writeFreelySchema } from "./schema";
import { getMenu } from "./menu";

let $title = document.querySelector("#title");
let $content = document.querySelector("#content");

// Bugs:
// 1. When there's just an empty line and a hard break is inserted with shift-enter then two enters are inserted
// which do not show up in the markdown ( maybe bc. they are training enters )

class ProseMirrorView {
  constructor(target, content) {
    let typingTimer;
    let localDraft = localStorage.getItem(window.draftKey);
    if (localDraft != null) {
      content = localDraft;
    }
    if (content.indexOf("# ") === 0) {
      let eol = content.indexOf("\n");
      let title = content.substring("# ".length, eol);
      content = content.substring(eol + "\n\n".length);
      $title.value = title;
    }

    const doc = writeFreelyMarkdownParser.parse(
      // Replace all "solo" \n's with \\\n for correct markdown parsing
      // Can't use lookahead or lookbehind because it's not supported on Safari
      content.replace(/([^]{0,1})(\n)([^]{0,1})/g, (match, p1, p2, p3) => {
        return p1 !== "\n" && p3 !== "\n" ? p1 + "\\\n" + p3 : match;
      })
    );

    this.view = new EditorView(target, {
      state: EditorState.create({
        doc,
        plugins: [
          keymap({
            "Mod-Enter": () => {
              document.getElementById("publish").click();
              return true;
            },
            "Mod-k": () => {
              const linkButton = document.querySelector(
                ".ProseMirror-icon[title='Add or remove link']"
              );
              linkButton.dispatchEvent(new Event("mousedown"));
              return true;
            },
          }),
          ...exampleSetup({
            schema: writeFreelySchema,
            menuContent: getMenu(),
          }),
        ],
      }),
      dispatchTransaction(transaction) {
        let newState = this.state.apply(transaction);
        const newContent = writeFreelyMarkdownSerializer
          .serialize(newState.doc)
          // Replace all \\\ns ( not followed by a \n ) with \n
          .replace(/(\\\n)(\n{0,1})/g, (match, p1, p2) =>
            p2 !== "\n" ? "\n" + p2 : match
          );
        $content.value = newContent;
        let draft = "";
        if ($title.value != null && $title.value !== "") {
          draft = "# " + $title.value + "\n\n";
        }
        draft += newContent;
        clearTimeout(typingTimer);
        typingTimer = setTimeout(doneTyping, doneTypingInterval);
        this.updateState(newState);
      },
    });
    // Editor is focused to the last position. This is a workaround for a bug:
    // 1. 1 type something in an existing entry
    // 2. reload - works fine, the draft is reloaded
    // 3. reload again - the draft is somehow removed from localStorage and the original content is loaded
    // When the editor is focused the content is re-saved to localStorage

    // This is also useful for editing, so it's not a bad thing even
    const lastPosition = this.view.state.doc.content.size;
    const selection = TextSelection.create(this.view.state.doc, lastPosition);
    this.view.dispatch(this.view.state.tr.setSelection(selection));
    this.view.focus();
  }

  get content() {
    return writeFreelyMarkdownSerializer.serialize(this.view.state.doc);
  }
  focus() {
    this.view.focus();
  }
  destroy() {
    this.view.destroy();
  }
}

let place = document.querySelector("#editor");
let view = new ProseMirrorView(place, $content.value);
