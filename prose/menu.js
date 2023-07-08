import { MenuItem } from "prosemirror-menu";
import { buildMenuItems } from "prosemirror-example-setup";

import { writeFreelySchema } from "./schema";

function canInsert(state, nodeType, attrs) {
  let $from = state.selection.$from;
  for (let d = $from.depth; d >= 0; d--) {
    let index = $from.index(d);
    if ($from.node(d).canReplaceWith(index, index, nodeType, attrs))
      return true;
  }
  return false;
}

const ReadMoreItem = new MenuItem({
  label: "Read more",
  select: (state) => canInsert(state, writeFreelySchema.nodes.readmore),
  run(state, dispatch) {
    dispatch(
      state.tr.replaceSelectionWith(writeFreelySchema.nodes.readmore.create())
    );
  },
});

export const getMenu = () => {
  const menuContent = [
    ...buildMenuItems(writeFreelySchema).fullMenu,
    [ReadMoreItem],
  ];
  return menuContent;
};
