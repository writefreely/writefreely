#!/bin/bash
#
#	Copyright © 2020 A Bunch Tell LLC.
#
#	This file is part of WriteFreely.
#
#	WriteFreely is free software: you can redistribute it and/or modify
#	it under the terms of the GNU Affero General Public License, included
#	in the LICENSE file in this source code package.
#
###############################################################################
#
# WriteFreely CSS invalidation script
#
# usage: ./invalidate-css.sh <build-directory>
#
# This script provides an automated way to invalidate stylesheets cached in the
# browser. It uses the last git commit hashes of the most frequently modified
# LESS files in the project and appends them to the stylesheet `href` in all
# template files.
#
# This is designed to be used when building a WriteFreely release.
#
###############################################################################

# Get parent build directory from first argument
buildDir=$1

# Get short hash of each primary LESS file's last commit
cssHash=$(git log -n 1 --pretty=format:%h -- less/core.less)
cssNewHash=$(git log -n 1 --pretty=format:%h -- less/new-core.less)
cssPadHash=$(git log -n 1 --pretty=format:%h -- less/pad.less)

echo "Adding write.css version ($cssHash $cssNewHash $cssPadHash) to .tmpl files..."
cd "$buildDir/templates" || exit 1
find . -type f -name "*.tmpl" -print0 | xargs -0 sed -i "s/write.css/write.css?${cssHash}${cssNewHash}${cssPadHash}/g"
find . -type f -name "*.tmpl" -print0 | xargs -0 sed -i "s/{{.Theme}}.css/{{.Theme}}.css?${cssHash}${cssNewHash}${cssPadHash}/g"