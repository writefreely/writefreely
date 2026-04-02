import { MarkdownParser } from "prosemirror-markdown";
import markdownit from "markdown-it";

import { writeFreelySchema } from "./schema";

const md = markdownit("commonmark", { html: true });

// Map HTML comment shortcodes to their own token types so they are handled
// as special blocks rather than generic HTML.
const SHORTCODE_TOKENS = {
  "<!--more-->": "readmore_block",
  "<!--emailsub-->": "emailsub_block",
};

md.core.ruler.push("shortcodes", (state) => {
  for (let i = 0; i < state.tokens.length; i++) {
    const token = state.tokens[i];
    if (token.type === "html_block") {
      const mapped = SHORTCODE_TOKENS[token.content.trim()];
      if (mapped) token.type = mapped;
    }
  }
});

export const writeFreelyMarkdownParser = new MarkdownParser(
  writeFreelySchema,
  md,
  {
    blockquote: { block: "blockquote" },
    paragraph: { block: "paragraph" },
    list_item: { block: "list_item" },
    bullet_list: { block: "bullet_list" },
    ordered_list: {
      block: "ordered_list",
      getAttrs: (tok) => ({ order: +tok.attrGet("start") || 1 }),
    },
    heading: {
      block: "heading",
      getAttrs: (tok) => ({ level: +tok.tag.slice(1) }),
    },
    code_block: { block: "code_block", noCloseToken: true },
    fence: {
      block: "code_block",
      getAttrs: (tok) => ({ params: tok.info || "" }),
      noCloseToken: true,
    },
    hr: { node: "horizontal_rule" },
    image: {
      node: "image",
      getAttrs: (tok) => ({
        src: tok.attrGet("src"),
        title: tok.attrGet("title") || null,
        alt: (tok.children !== null && typeof tok.children[0] !== 'undefined' ? tok.children[0].content : null),
      }),
    },
    hardbreak: { node: "hard_break" },

    em: { mark: "em" },
    strong: { mark: "strong" },
    link: {
      mark: "link",
      getAttrs: (tok) => ({
        href: tok.attrGet("href"),
        title: tok.attrGet("title") || null,
      }),
    },
    code_inline: { mark: "code", noCloseToken: true },
    readmore_block: { node: "readmore" },
    emailsub_block: { node: "emailsub" },
    html_block: {
      node: "html_block",
      getAttrs: (tok) => ({ content: tok.content }),
    },
    html_inline: {
      node: "html_inline",
      getAttrs: (tok) => ({ content: tok.content }),
    },
  }
);
