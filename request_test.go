package writefreely

import (
	"net/http/httptest"
	"testing"
)

func TestIsActivityPubRequest(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{
			name:   "activityjson accepted",
			accept: "application/activity+json",
			want:   true,
		},
		{
			name:   "ldjson with profile exact",
			accept: "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"",
			want:   true,
		},
		{
			name:   "ldjson with profile among multiple accept values",
			accept: "application/ld+json;profile=\"https://www.w3.org/ns/activitystreams\", application/activity+json",
			want:   true,
		},
		{
			name:   "ldjson without activitystreams profile",
			accept: "application/ld+json",
			want:   false,
		},
		{
			name:   "plain json should not be treated as activitypub",
			accept: "application/json",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept", tt.accept)

			got := IsActivityPubRequest(req)
			if got != tt.want {
				t.Fatalf("IsActivityPubRequest() = %v, want %v (Accept: %q)", got, tt.want, tt.accept)
			}
		})
	}
}
