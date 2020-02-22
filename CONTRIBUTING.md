# Contributing to WriteFreely

Welcome! We're glad you're interested in contributing to WriteFreely.

For **questions**, **help**, **feature requests**, and **general discussion**, please use [our forum](https://discuss.write.as).

For **bug reports**, please [open a GitHub issue](https://github.com/writeas/writefreely/issues/new). See our guide on [submitting bug reports](https://writefreely.org/contribute#bugs).

## Getting Started

There are many ways to contribute to WriteFreely, from code to documentation, to translations, to help in the community!

See our [Contributing Guide](https://writefreely.org/contribute) on WriteFreely.org for ways to contribute without writing code. Otherwise, please read on.

## Working on WriteFreely

First, you'll want to clone the WriteFreely repo, install development dependencies, and build the application from source. Learn how to do this in our [Development Setup](https://writefreely.org/docs/latest/developer/setup) guide.

### Starting development

Next, [join our forum](https://discuss.write.as) so you can discuss development with the team. Then take a look at [our roadmap on Phabricator](https://phabricator.write.as/tag/write_freely/) to see where the project is today and where it's headed.

When you find something you want to work on, start a new topic on the forum or jump into an existing discussion, if there is one. The team will respond and continue the conversation there.

Lastly, **before submitting any code**, please sign our [contributor's agreement](https://phabricator.write.as/L1) so we can accept your contributions. It is substantially similar to the _Apache Individual Contributor License Agreement_. If you'd like to know about the rationale behind this requirement, you can [read more about that here](https://phabricator.write.as/w/writefreely/cla/).

### Branching

All stable work lives on the `master` branch. We merge into it only when creating a release. Releases are tagged using semantic versioning.

While developing, we primarily work from the `develop` branch, creating _feature branches_ off of it for new features and fixes. When starting a new feature or fix, you should also create a new branch off of `develop`.

#### Branch naming

For fixes and modifications to existing behavior, branch names should follow a similar pattern to commit messages (see below), such as `fix-post-rendering` or `update-documentation`. You can optionally append a task number, e.g. `fix-post-rendering-T000`.

For new features, branches can be named after the new feature, e.g. `activitypub-mentions` or `import-zip`.

#### Pull request scope

The scope of work on each branch should be as small as possible -- one complete feature, one complete change, or one complete fix. This makes it easier for us to review and accept.

### Writing code

We value reliable, readable, and maintainable code over all else in our work. To help you write that kind of code, we offer a few guiding principles, as well as a few concrete guidelines.

#### Guiding principles

* Write code for other humans, not computers.
* The less complexity, the better. The more someone can understand code just by looking at it, the better.
* Functionality, readability, and maintainability over senseless elegance.
* Only abstract when necessary. 
* Keep an eye to the future, but don't pre-optimize at the expense of today's simplicity.

#### Code guidelines

* Format all Go code with `go fmt` before committing (**important!**)
* Follow whitespace conventions established within the project (tabs vs. spaces)
* Add comments to exported Go functions and variables
* Follow Go naming conventions, like using [`mixedCaps`](https://golang.org/doc/effective_go.html#mixed-caps)
* Avoid new dependencies unless absolutely necessary

### Commit messages

We highly value commit messages that follow established form within the project. Generally speaking, we follow the practices [outlined](https://git-scm.com/book/en/v2/Distributed-Git-Contributing-to-a-Project#_commit_guidelines) in the Pro Git Book. A good commit message will look like the following:

* **Line 1**: A short summary written in the present imperative tense. For example:
  * ✔️ **Good**: "Fix post rendering bug"
  * ❌ No: ~~"Fixes post rendering bug"~~
  * ❌ No: ~~"Fixing post rendering bug"~~
  * ❌ No: ~~"Fixed post rendering bug"~~
  * ❌ No: ~~"Post rendering bug is fixed now"~~
* **Line 2**: _[left blank]_
* **Line 3**: An added description of what changed, any rationale, etc. -- if necessary
* **Last line**: A mention of any applicable task or issue
  * For Phabricator tasks: `Ref T000` or `Closes T000`
  * For GitHub issues: `Ref #000` or `Fixes #000`

#### Good examples

When in doubt, look to our existing git history for examples of good commit messages. Here are a few:

* [Rename Suspend status to Silence](https://github.com/writeas/writefreely/commit/7e014ca65958750ab703e317b1ce8cfc4aad2d6e)
* [Show 404 when remote user not found](https://github.com/writeas/writefreely/commit/867eb53b3596bd7b3f2be3c53a3faf857f4cd36d)
* [Fix post deletion on Pleroma](https://github.com/writeas/writefreely/commit/fe82cbb96e3d5c57cfde0db76c28c4ea6dabfe50)

### Submitting pull requests

Like our GitHub issues, we aim to keep our number of open pull requests to a minimum. You can follow a few guidelines to ensure changes are merged quickly.

First, make sure your changes follow the established practices and good form outlined in this guide. This is crucial to our project, and ignoring our practices can delay otherwise important fixes.

Beyond that, we prioritize pull requests in this order:

1. Fixes to open GitHub issues
2. Superficial changes and improvements that don't adversely impact users
3. New features and changes that have been discussed before with the team

Any pull requests that haven't previously been discussed with the team may be extensively delayed or closed, especially if they require a wider consideration before integrating into the project. When in doubt, please reach out [on the forum](https://discuss.write.as) before submitting a pull request.