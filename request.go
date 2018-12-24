/*
 * Copyright © 2018 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */
package writefreely

import "mime"

func IsJSON(h string) bool {
	ct, _, _ := mime.ParseMediaType(h)
	return ct == "application/json"
}
