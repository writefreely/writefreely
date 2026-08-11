package writefreely

import (
	"fmt"
	"testing"

	"github.com/writeas/web-core/activitystreams"
)

var actorTestTable = []struct {
	Name string
	Resp []byte
}{
	{
		"Context as a string",
		[]byte(`{"@context":"https://www.w3.org/ns/activitystreams"}`),
	},
	{
		"Context as a list",
		[]byte(`{"@context":["one string", "two strings"]}`),
	},
}

func TestUnmarshalActor(t *testing.T) {
	for _, tc := range actorTestTable {
		actor := activitystreams.Person{}
		err := unmarshalActor(tc.Resp, &actor)
		if err != nil {
			t.Errorf("%s failed with error %s", tc.Name, err)
		}
	}
}

func TestAPCollectionPage(t *testing.T) {
	const root = "https://example.com/api/collections/blog"

	makeActors := func(n int) []string {
		actors := make([]string, n)
		for i := 0; i < n; i++ {
			actors[i] = fmt.Sprintf("https://remote.example/users/%d", i)
		}
		return actors
	}

	t.Run("full first page with more to come", func(t *testing.T) {
		actors := makeActors(apFollowersPageSize + 5)
		ocp := apCollectionPage(root, "followers", actors, 1)

		if got := len(ocp.OrderedItems); got != apFollowersPageSize {
			t.Errorf("expected %d items on page 1, got %d", apFollowersPageSize, got)
		}
		if ocp.OrderedItems[0] != actors[0] {
			t.Errorf("expected first item %q, got %q", actors[0], ocp.OrderedItems[0])
		}
		wantNext := fmt.Sprintf("%s/followers?page=2", root)
		if ocp.Next != wantNext {
			t.Errorf("expected Next %q, got %q", wantNext, ocp.Next)
		}
		if ocp.Prev != "" {
			t.Errorf("expected no Prev on page 1, got %q", ocp.Prev)
		}
		if ocp.PartOf != root+"/followers" {
			t.Errorf("expected PartOf %q, got %q", root+"/followers", ocp.PartOf)
		}
		if ocp.TotalItems != len(actors) {
			t.Errorf("expected TotalItems %d, got %d", len(actors), ocp.TotalItems)
		}
	})

	t.Run("last page clears Next and sets Prev", func(t *testing.T) {
		actors := makeActors(apFollowersPageSize + 5)
		ocp := apCollectionPage(root, "followers", actors, 2)

		if got := len(ocp.OrderedItems); got != 5 {
			t.Errorf("expected 5 items on last page, got %d", got)
		}
		if ocp.Next != "" {
			t.Errorf("expected no Next on last page, got %q", ocp.Next)
		}
		wantPrev := fmt.Sprintf("%s/followers?page=1", root)
		if ocp.Prev != wantPrev {
			t.Errorf("expected Prev %q, got %q", wantPrev, ocp.Prev)
		}
	})

	t.Run("out-of-range page is empty with no Next", func(t *testing.T) {
		actors := makeActors(3)
		ocp := apCollectionPage(root, "followers", actors, 5)

		if len(ocp.OrderedItems) != 0 {
			t.Errorf("expected no items on out-of-range page, got %d", len(ocp.OrderedItems))
		}
		if ocp.Next != "" {
			t.Errorf("expected no Next on out-of-range page, got %q", ocp.Next)
		}
	})

	t.Run("empty collection", func(t *testing.T) {
		ocp := apCollectionPage(root, "followers", []string{}, 1)

		if len(ocp.OrderedItems) != 0 {
			t.Errorf("expected no items, got %d", len(ocp.OrderedItems))
		}
		if ocp.Next != "" {
			t.Errorf("expected no Next on empty collection, got %q", ocp.Next)
		}
	})
}
