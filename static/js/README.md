# static/js

This directory is for Javascript.

## Updating libraries

Update instructions, for libraries that involve more than just downloading the latest version.

### highlightjs

To update the highlightjs library, first download a plain package (no languages included) [from highlightjs.org](https://highlightjs.org/download/). The `highlight.pack.js` file in the archive should be moved into this `static/js/` directory and renamed to `highlight.min.js`.

Then [download an archive](https://github.com/highlightjs/highlight.js/releases) of the latest version. Extract it to some directory, and replace **~/Downloads/highlight.js** below with the resulting directory.

```bash
#!/bin/bash

version=9.13.1

cd $GOPATH/src/github.com/writeas/writefreely/static/js/highlightjs
for f in $(ls ~/Downloads/highlight.js/src/languages); do
	# Use minified versions
	f=$(echo $f | sed 's/\.js/.min.js/')
	# Download the version
	wget "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/$version/languages/$f"
done
```

Commit the changes and you're done!
