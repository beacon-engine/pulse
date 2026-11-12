# Releasing

The version lives in [`VERSION`](../VERSION) (`X.Y.Z`, no `v`). It is built into
both binaries and the dashboard, and becomes the Docker image tag.

## Branches

```
feature branch ──PR──► release/vX.Y.Z ──PR──► master ──► images + vX.Y.Z tag
                       (or hotfix/vX.Y.Z)
```

`master` only takes release and hotfix branches. Feature branches merge into a
release branch.

## Steps

1. **Cut the release branch and set the version**, then commit:
   ```bash
   make version V=0.2.0 BRANCH=release   # or BRANCH=hotfix
   git commit -am "chore: version 0.2.0"
   git push -u origin release/v0.2.0
   ```
   [`scripts/set-version.sh`](../scripts/set-version.sh) writes `VERSION` and
   the dashboard's `package.json` / `package-lock.json`. It refuses a version
   that is already tagged, or a branch that already exists. Without `BRANCH` it
   only updates the files on the current branch.
2. **Merge features into it through PRs.** Every PR into a release or hotfix
   branch runs the tests, a Docker build, and the security scans.
3. **Open a PR from the release branch into `master`.** CI also checks that the
   branch name, `VERSION` and the dashboard's version files agree, and that the
   version isn't released yet.
4. **Merge.** The release workflow pushes `beaconengine/pulse-gateway` and
   `beaconengine/pulse-agent` tagged `X.Y.Z`, `X.Y`, `latest` and
   `sha-<commit>`, then tags the commit `vX.Y.Z` and creates a GitHub release
   listing the images. It doesn't rescan: the code was scanned on its way into
   the release branch.

A push to `master` whose version is already tagged only refreshes `latest` and
`sha-<commit>`, and those builds report `X.Y.Z-dev+<commit>`.

## CI workflows

| Workflow | Runs on | Does |
|---|---|---|
| `ci.yml` | PRs into `master`, `release/**`, `hotfix/**` | Go format, vet and tests; dashboard build; Docker build. PRs into `master` also get the release-branch checks. |
| `security.yml` | PRs into `release/**`, `hotfix/**`; weekly on `master`; on demand | Checkov and Trivy ([security.md](security.md#scanning-the-code)) |
| `release.yml` | Pushes to `master` | Builds and pushes the images; tags and publishes new versions |

The release job needs **Settings → Actions → General → Workflow permissions**
to allow write access, to create the tag and release. Docker Hub credentials
come from the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` repository secrets.

Images are built for `linux/amd64` only.
