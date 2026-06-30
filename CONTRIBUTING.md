# Contributing to SkyPanel

First off, thank you for considering contributing to SkyPanel! It's people like you that make SkyPanel such a great tool.

## Where do I go from here?

If you've noticed a bug or have a feature request, make sure to check our [Issues](https://github.com/Aether-Panel/Panel/issues) to see if someone else has already created a ticket. If not, go ahead and make one!

## Fork & create a branch

If this is something you think you can fix, then fork SkyPanel and create a branch with a descriptive name.

A good branch name would be (where issue #325 is the ticket you're working on):

```sh
git checkout -b 325-add-new-server-feature
```

## Implementation Guidelines

- Ensure you adhere to the standard Go formatting. Run `go fmt ./...` before committing.
- Ensure your code passes the test suite by running `go test ./...`.
- Add tests for any new features or bug fixes.

## Pull Requests

Once you're ready, submit a Pull Request! We use a standard Pull Request template to help us review your code quickly. Please fill it out as completely as possible.
