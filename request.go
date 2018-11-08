package writefreely

import "mime"

func IsJSON(h string) bool {
	ct, _, _ := mime.ParseMediaType(h)
	return ct == "application/json"
}
