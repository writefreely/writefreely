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
  const tokens = state.tokens;
  // Iterate backwards so splicing at index i doesn't shift unvisited indices.
  for (let i = tokens.length - 1; i >= 0; i--) {
    const token = tokens[i];

    if (token.type === "html_block") {
      const content = token.content.trim();
      const mapped = SHORTCODE_TOKENS[content];
      if (mapped) {
        token.type = mapped;
        continue;
      }
      /* NOTE: future discussion support
      // comment is an inline node — when it appears as a standalone html_block,
      // wrap it in a paragraph so ProseMirror can place the inline node correctly.
      if (content === "<!--comment-->") {
        const open = new state.Token("paragraph_open", "p", 1);
        const inlineTok = new state.Token("inline", "", 0);
        inlineTok.children = [new state.Token("comment_token", "", 0)];
        const close = new state.Token("paragraph_close", "p", -1);
        tokens.splice(i, 1, open, inlineTok, close);
      }
       */
      continue;
    }

    /* NOTE: future discussion support
    // Handle <!--comment--> appearing as html_inline inside an existing paragraph.
    if (token.type === "inline" && token.children) {
      for (const child of token.children) {
        if (child.type === "html_inline" && child.content.trim() === "<!--comment-->") {
          child.type = "comment_token";
        }
      }
    }
     */
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
    comment_token: { node: "comment" },
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
