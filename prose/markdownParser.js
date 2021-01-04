import { MarkdownParser } from "prosemirror-markdown";
import markdownit from "markdown-it";

import { writeFreelySchema } from "./schema";

export const writeAsMarkdownParser = new MarkdownParser(
    writeFreelySchema,
    markdownit("commonmark", { html: true }),
    {
        // blockquote: { block: "blockquote" },
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
        // hr: { node: "horizontal_rule" },
        image: {
            node: "image",
            getAttrs: (tok) => ({
                src: tok.attrGet("src"),
                title: tok.attrGet("title") || null,
                alt: tok.children?.[0].content || null,
            }),
        },
        // hardbreak: { node: "hard_break" },

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
        html_block: {
            node: "readmore",
            getAttrs(token) {
                console.log({ token });
                // TODO: Give different attributes depending on the token content
                return {};
            },
        },
    },
);
