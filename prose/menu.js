import { MenuItem } from "prosemirror-menu";
import { buildMenuItems } from "prosemirror-example-setup";
import { NodeSelection } from "prosemirror-state";

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
  const builtItems = buildMenuItems(writeFreelySchema);
  const { toggleLink } = builtItems;

  const patchedLink = new MenuItem({
    ...toggleLink.spec,
    select(state) {
      if (
        state.selection instanceof NodeSelection &&
        state.selection.node.type === writeFreelySchema.nodes.comment
      ) {
        return false;
      }
      return toggleLink.spec.select ? toggleLink.spec.select(state) : true;
    },
  });

  const fullMenu = builtItems.fullMenu.map((group) =>
    group.map((item) => (item === toggleLink ? patchedLink : item))
  );

  return [...fullMenu, [ReadMoreItem]];
};
