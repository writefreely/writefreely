&nbsp;
<p align="center">
	<a href="https://writefreely.org"><img src="https://writefreely.org/img/writefreely.svg" width="350px" alt="Write Freely" /></a>
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
	<a href="https://github.com/writeas/writefreely/releases/latest">
		<img src="https://img.shields.io/github/downloads/writeas/writefreely/total.svg" />
	</a>
	<a href="https://hub.docker.com/r/writeas/writefreely/">
		<img src="https://img.shields.io/docker/pulls/writeas/writefreely.svg" />
	</a>
</p>
&nbsp;

WriteFreely is a beautifully pared-down blogging platform that's simple on the surface, yet powerful underneath.

It's designed to be flexible and share your writing widely, so it's built around plain text and can publish to the _fediverse_ via ActivityPub. It's easy to install and light enough to run on a Raspberry Pi.

**[Start a blog on our instance](https://write.as/new/blog/federated)**

[Try the editor](https://write.as/new)

[Find another instance](https://writefreely.org/instances)

## Features

* Start a blog for yourself, or host a community of writers
* Form larger federated networks, and interact over modern protocols like ActivityPub
* Write on a fast, dead-simple, and distraction-free editor
* Format text with Markdown, and organize posts with hashtags
* Publish drafts and let others proofread them by sharing a private link
* Create multiple lightweight blogs under a single account
* Export all data in plain text files
* Read a stream of other posts in your writing community
* Build more advanced apps and extensions with the [well-documented API](https://developers.write.as/docs/api/)
* Designed around user privacy and consent

## Quick start

WriteFreely has minimal requirements to get up and running — you only need to be able to run an executable.

> **Note** this is currently alpha software. We're quickly moving out of this v0.x stage, but while we're in it, there are no guarantees that this is ready for production use.

First, download the [latest release](https://github.com/writeas/writefreely/releases/latest) for your OS. It includes everything you need to start your blog.

Now extract the files from the archive, change into the directory, and do the following steps:

```bash
# 1) Configure your blog
./writefreely --config

# 2) (if you chose MySQL in the previous step) Log into MySQL and run:
# CREATE DATABASE writefreely;

# 3) Import the schema with:
./writefreely --init-db

# 4) Generate data encryption keys
./writefreely --gen-keys

# 5) Run
./writefreely

# 6) Check out your site at the URL you specified in the setup process
# 7) There is no Step 7, you're done!
```

For running in production, [see our guide](https://writefreely.org/start#production).

## Packages

WriteFreely is available in these package repositories:

* [Arch User Repository](https://aur.archlinux.org/packages/writefreely/)

## Development

Ready to hack on your site? Here's a quick overview.

### Prerequisites

* [Go 1.10+](https://golang.org/dl/)
* [Node.js](https://nodejs.org/en/download/)

### Setting up

```bash
go get github.com/writeas/writefreely/cmd/writefreely
```

Configure your site, create your database, and import the schema [as shown above](#quick-start). Then generate the remaining files you'll need:

```bash
make install # Generates encryption keys; installs LESS compiler
make ui      # Generates CSS (run this whenever you update your styles)
make run     # Runs the application
```

## Docker

### Using Docker for Development

If you'd like to use Docker as a base for working on a site's styles and such,
you can run the following from a Bash shell.

*Note: This process is intended only for working on site styling. If you'd
like to run Write Freely in production as a Docker service, it'll require a
little more work.*

The `docker-setup.sh` script will present you with a few questions to set up
your dev instance. You can hit enter for most of them, except for "Admin username"
and "Admin password." You'll probably have to wait a few seconds after running
`docker-compose up -d` for the Docker services to come up before running the
bash script.

```
docker-compose up -d
./docker-setup.sh
```

Now you should be able to navigate to http://localhost:8080 and start working!

When you're completely done working, you can run `docker-compose down` to destroy
your virtual environment, including your database data. Otherwise, `docker-compose stop`
will shut down your environment without destroying your data.

### Using Docker for Production

Write Freely doesn't yet provide an official Docker pathway to production. We're
working on it, though!

## Contributing

We gladly welcome contributions to WriteFreely, whether in the form of [code](https://github.com/writeas/writefreely/blob/master/CONTRIBUTING.md#contributing-to-writefreely), [bug reports](https://github.com/writeas/writefreely/issues/new?template=bug_report.md), [feature requests](https://discuss.write.as/c/feedback/feature-requests), [translations](https://poeditor.com/join/project/TIZ6HFRFdE), or documentation improvements.

Before contributing anything, please read our [Contributing Guide](https://github.com/writeas/writefreely/blob/master/CONTRIBUTING.md#contributing-to-writefreely). It describes the correct channels for submitting contributions and any potential requirements.

## License

Licensed under the AGPL.
