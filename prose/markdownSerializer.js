import { MarkdownSerializer } from "prosemirror-markdown";

function backticksFor(node, side) {
  const ticks = /`+/g;
  let m;
  let len = 0;
  if (node.isText)
    while ((m = ticks.exec(node.text))) len = Math.max(len, m[0].length);
  let result = len > 0 && side > 0 ? " `" : "`";
  for (let i = 0; i < len; i++) result += "`";
  if (len > 0 && side < 0) result += " ";
  return result;
}

function isPlainURL(link, parent, index, side) {
  if (link.attrs.title || !/^\w+:/.test(link.attrs.href)) return false;
  const content = parent.child(index + (side < 0 ? -1 : 0));
  if (
    !content.isText ||
    content.text != link.attrs.href ||
    content.marks[content.marks.length - 1] != link
  )
    return false;
  if (index == (side < 0 ? 1 : parent.childCount - 1)) return true;
  const next = parent.child(index + (side < 0 ? -2 : 1));
  return !link.isInSet(next.marks);
}

export const writeAsMarkdownSerializer = new MarkdownSerializer(
  {
    readmore(state, node) {
      state.write("<!--more-->\n");
      state.closeBlock(node);
    },
    // blockquote(state, node) {
    //   state.wrapBlock("> ", undefined, node, () => state.renderContent(node));
    // },
    code_block(state, node) {
      state.write(`\`\`\`${node.attrs.params || ""}\n`);
      state.text(node.textContent, false);
      state.ensureNewLine();
      state.write("```");
      state.closeBlock(node);
    },
    heading(state, node) {
      state.write(`${state.repeat("#", node.attrs.level)} `);
      state.renderInline(node);
      state.closeBlock(node);
    },
    // horizontal_rule(state, node) {
    //   state.write(node.attrs.markup || "---");
    //   state.closeBlock(node);
    // },
    bullet_list(state, node) {
      state.renderList(node, "  ", () => `${node.attrs.bullet || "*"} `);
    },
    ordered_list(state, node) {
      const start = node.attrs.order || 1;
      const maxW = String(start + node.childCount - 1).length;
      const space = state.repeat(" ", maxW + 2);
      state.renderList(node, space, (i) => {
        const nStr = String(start + i);
        return `${state.repeat(" ", maxW - nStr.length) + nStr}. `;
      });
    },
    list_item(state, node) {
      state.renderContent(node);
    },
    paragraph(state, node) {
      state.renderInline(node);
      state.closeBlock(node);
    },

    image(state, node) {
      state.write(
        `![${state.esc(node.attrs.alt || "")}](${state.esc(node.attrs.src)}${
          node.attrs.title ? ` ${state.quote(node.attrs.title)}` : ""
        })`,
      );
    },
    hard_break(state, node, parent, index) {
      for (let i = index + 1; i < parent.childCount; i += 1)
        if (parent.child(i).type !== node.type) {
          state.write("\n");
          return;
        }
    },
    text(state, node) {
      state.text(node.text || "");
    },
  },
  {
    em: {
      open: "*",
      close: "*",
      mixable: true,
      expelEnclosingWhitespace: true,
    },
    strong: {
      open: "**",
      close: "**",
      mixable: true,
      expelEnclosingWhitespace: true,
    },
    link: {
      open(_state, mark, parent, index) {
        return isPlainURL(mark, parent, index, 1) ? "<" : "[";
      },
      close(state, mark, parent, index) {
        return isPlainURL(mark, parent, index, -1)
          ? ">"
          : `](${state.esc(mark.attrs.href)}${
              mark.attrs.title ? ` ${state.quote(mark.attrs.title)}` : ""
            })`;
      },
    },
    code: {
      open(_state, _mark, parent, index) {
        return backticksFor(parent.child(index), -1);
      },
      close(_state, _mark, parent, index) {
        return backticksFor(parent.child(index - 1), 1);
      },
      escape: false,
    },
  },
);
