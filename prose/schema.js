import { schema } from "prosemirror-markdown";
import { Schema } from "prosemirror-model";

export const writeFreelySchema = new Schema({
  nodes: schema.spec.nodes
    .remove("blockquote")
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
    }),
  marks: schema.spec.marks,
});
