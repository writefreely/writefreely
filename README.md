&nbsp;
<p align="center">
	<a href="https://writefreely.org"><img src="https://writefreely.org/writefreely.svg" width="350px" alt="Write Freely" /></a>
</p>
<hr />
<p align="center">
	<a href="https://github.com/writeas/writefreely/releases/">
		<img src="https://img.shields.io/github/release/writeas/writefreely.svg" alt="Latest release" />
	</a>
	<a href="https://goreportcard.com/report/github.com/writeas/writefreely">
		<img src="https://goreportcard.com/badge/github.com/writeas/writefreely" alt="Go Report Card" />
	</a>
	<a href="https://travis-ci.org/writeas/writefreely">
		<img src="https://travis-ci.org/writeas/writefreely.svg" alt="Build status" />
	</a>
</p>
&nbsp;

WriteFreely is a beautifully pared-down blogging platform that's simple on the surface, yet powerful underneath.

It's designed to be flexible and share your writing widely, so it's built around plain text and can publish to the _fediverse_ via ActivityPub. It's easy to install and lightweight.

**Note** this is currently alpha software. We're quickly moving out of this v0.x stage, but while we're in it, there are no guarantees that this is ready for production use.

## Features

* Start a blog for yourself, or host a community of writers
* Form larger federated networks, and interact over modern protocols like ActivityPub
* Write on a dead-simple, distraction-free and super fast editor
* Publish drafts and let others proofread them by sharing a private link
* Build more advanced apps and extensions with the [well-documented API](https://developers.write.as/docs/api/)

## Quick start

First, download the [latest release](https://github.com/writeas/writefreely/releases/latest) for your OS. It includes everything you need to start your blog.

Now extract the files from the archive, change into the directory, and do the following steps:

```bash
# 1) Log into MySQL and run:
# CREATE DATABASE writefreely;
#
# 2) Import the schema with:
mysql -u YOURUSERNAME -p writefreely < schema.sql

# 3) Configure your blog
./writefreely --config

# 4) Generate data encryption keys (especially for production)
./keys.sh

# 5) Run
./writefreely

# 6) Check out your site at the URL you specified in the setup process
# 7) There is no Step 7, you're done!
```

## Development

Ready to hack on your site? Here's a quick overview.

### Prerequisites

* [Go 1.10+](https://golang.org/dl/)
* [Node.js](https://nodejs.org/en/download/)

### Setting up

```bash
go get github.com/writeas/writefreely/cmd/writefreely
```

Create your database, import the schema, and configure your site [as shown above](#quick-start).

Now generate the CSS:

```bash
make install # Generates encryption keys; installs LESS compiler
make ui      # Generates CSS (run this whenever you update your styles)
make run     # Runs the application
```

## License

Licensed under the AGPL.
