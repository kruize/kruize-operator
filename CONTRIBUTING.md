# Contributing to Kruize Operator

Thank you for your interest in the Kruize Operator! We welcome your contributions to the project.

## Submitting a Contribution

The Kruize Operator is [Apache 2.0](https://github.com/kruize/kruize-operator/blob/main/LICENSE) licensed and uses GitHub to manage reviews of pull requests. This document outlines the contribution workflow, commit message formatting, contact details and other resources to make it easier to get your contribution accepted.

### Contribution Workflow

These steps outline the general contribution workflow:

- Fork and set up the [kruize-operator](https://github.com/kruize/kruize-operator) repository on your workstation
- Create a topic branch based off the `mvp_demo` branch
- Follow the coding guidelines while making code changes
- Install the required software to build and deploy the operator (see [Prerequisites](#prerequisites))
- Make sure the tests pass, and add any new tests if appropriate (see [Testing](#testing))
- Make commits of logical units
- Make sure your commit messages are in the proper format (see [Commit Message](#commit-message))
- Push your changes to a topic branch in your fork of the repository
- Submit a pull request to the original repository
- Ensure the PR checks pass (see [PR Checks](#pr-checks))
- For any queries, open a [GitHub Discussion](https://github.com/kruize/kruize-operator/discussions) or an [issue](https://github.com/kruize/kruize-operator/issues)

If this is your first pull request on GitHub, click [here](https://www.freecodecamp.org/news/how-to-make-your-first-pull-request-on-github-3/) to quickly get started.

---

## Prerequisites

See the [Prerequisites section in README.md](README.md#prerequisites) for the full list of required tools and versions.

---

## Building

```sh
# Generate code and manifests
make generate manifests

# Build the operator image
make docker-build IMG=<registry>/kruize-operator:tag

# Build and push
make docker-build docker-push IMG=<registry>/kruize-operator:tag
```

For a streamlined build and push workflow with prerequisite checks and version management, use the provided script:

```sh
./scripts/operator_build_and_push.sh -o <operator_image> -b <bundle_image>
```

---

## Testing

Before submitting a pull request, ensure all tests pass:

```sh
# Run unit tests
make test

# Run end-to-end tests (requires a running cluster)
make test-e2e
```

See [test/Operator_tests.md](test/Operator_tests.md) for detailed testing documentation.

---

## PR Checks

All pull requests are subject to automated checks defined in [`.github/workflows/pr-check.yaml`](.github/workflows/pr-check.yaml). Ensure:

- Unit tests pass (`make test`)
- Code compiles without errors (`make build` or `make generate manifests`)
- No linting errors

---

## Commit Message

We use GPG keys for signing commits. Refer to [Generating a new GPG key](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key) for details.

The commit message should indicate **what** has changed and **why** the change was made. Sign off on your commit in the footer. By doing this, you assert original authorship of the commit and that you are permitted to contribute it.

This can be automatically added to your commit by passing `-S` (GPG sign) and `-s` (sign-off) to `git commit`:

```sh
git commit -S -s -m "your commit message"
```

Or by manually adding the following line to the footer of the commit:

```
Signed-off-by: Full Name <email>
```

### Commit Message Format

```
<type>: <short summary>

<optional body explaining why the change was made>

Signed-off-by: Full Name <email>
```

**Types:** `fix`, `feat`, `docs`, `refactor`, `test`, `chore`, `ci`

**Examples:**

```
fix: correct finalizer cleanup timeout handling

The finalizer was not respecting the FINALIZER_TIMEOUT_SECONDS env var
during resource deletion, causing indefinite hangs on slow clusters.

Signed-off-by: Jane Doe <jane@example.com>
```

```
feat: add support for KIND cluster overlay

Signed-off-by: John Smith <john@example.com>
```

---

## Code Style and Guidelines

- Follow standard [Go conventions](https://google.github.io/styleguide/go/)
- Run `gofmt` before committing: `gofmt -w ./...`
- Keep changes focused — one logical change per commit and per PR
- Add or update documentation in `docs/` for non-trivial changes
- Do not hardcode secrets, image digests, or environment-specific values

---

## Reporting Issues

If you find a bug or want to request a feature, please [open an issue](https://github.com/kruize/kruize-operator/issues/new) with:

- A clear description of the problem or request
- Steps to reproduce (for bugs)
- The Kubernetes/OpenShift version and cluster type (Minikube, KIND, OpenShift)
- Relevant logs or error messages

---

## Additional Resources

- [Kruize Operator README](README.md)
- [Operator SDK Documentation](https://sdk.operatorframework.io/)
- [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager)

---

Thank you for contributing!
