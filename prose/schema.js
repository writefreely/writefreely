import { schema } from "prosemirror-markdown";
import { Schema } from "prosemirror-model";

export const writeFreelySchema = new Schema({
  nodes: schema.spec.nodes
    .addToEnd("readmore", {
      inline: false,
      content: "",
      group: "block",
      draggable: true,
      toDOM: (node) => [
        "div",
        { class: "editorreadmore" },
        "Read more...",
      ],
      parseDOM: [{ tag: "div.editorreadmore" }],
    })
    .addToEnd("emailsub", {
      inline: false,
      content: "",
      group: "block",
      draggable: true,
      toDOM: () => [
        "div", { id: "emailsub", contenteditable: "false" },
        ["form", {},
          ["p", {}, "Enter your email to subscribe to updates."],
          ["input", { type: "email", name: "email", placeholder: "me@example.com" }],
          ["input", { type: "submit", id: "subscribe-btn", value: "Subscribe" }],
        ],
      ],
      parseDOM: [{ tag: "div#emailsub" }],
    })
    .addToEnd("html_block", {
      attrs: { content: { default: "" } },
      content: "",
      group: "block",
      marks: "",
      draggable: true,
      toDOM: (node) => [
        "div",
        { class: "editor-html-block", contenteditable: "false" },
        node.attrs.content,
      ],
      parseDOM: [{ tag: "div.editor-html-block", getAttrs: (dom) => ({ content: dom.textContent }) }],
    })
    .addToEnd("html_inline", {
      attrs: { content: { default: "" } },
      inline: true,
      content: "",
      group: "inline",
      toDOM: (node) => ["code", { class: "editor-html-inline" }, node.attrs.content],
      parseDOM: [{ tag: "code.editor-html-inline", getAttrs: (dom) => ({ content: dom.textContent }) }],
    }),
  marks: schema.spec.marks,
});
