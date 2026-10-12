# Publishing a release

Releases are built and published by GitHub Actions ([release.yml](.github/workflows/release.yml)) when a `v*` tag is pushed. This is the checklist.

## Versioning

[Semantic Versioning](https://semver.org). While Designer is 0.x, a minor bump (0.2.0) may change Project file formats; say so in the changelog with what to do about it. A version with a suffix (`0.3.0-rc.1`) is published as a pre-release.

`VERSION` holds the version; the binary gets it at build time (`designer --version`).

## 1. Prepare

```sh
git switch main && git pull
make check            # frontend, vet, tests, third-party notices
make dist             # all platforms build; try the archive for your machine
```

Move the `Unreleased` entries in [CHANGELOG.md](CHANGELOG.md) under a new `## [x.y.z] - YYYY-MM-DD` heading, update the comparison links at the bottom, and commit.

If dependencies changed, `make notices` and commit `THIRD_PARTY_NOTICES.md` (CI fails otherwise).

## 2. Tag

```sh
make release V=x.y.z         # writes VERSION, commits "Release vx.y.z", tags vx.y.z
git push origin main vx.y.z
```

`make release` refuses to run with uncommitted changes or without a changelog section for the version.

Without `make` (e.g. Git Bash on Windows):

```sh
echo x.y.z > VERSION
git commit -am "Release vx.y.z"
git tag -a vx.y.z -m "Release vx.y.z"
git push origin main vx.y.z
```

## 3. What the workflow does

1. Checks the tag matches `VERSION` (and stops if not).
2. Builds the frontend, runs vet and the tests, checks the third-party notices.
3. Builds `designer_<version>_<os>_<arch>` archives for linux, darwin and windows on amd64 and arm64, each with `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md` and `README.md`, plus `checksums.txt`.
4. Signs build provenance for every archive.
5. Publishes the GitHub Release with the changelog section as its notes.

If it fails after the tag is pushed, fix the problem on `main`, then either delete and re-push the tag, or run the workflow by hand (**Actions → Release → Run workflow**) with the tag name.

## 4. Check the release

- The [Releases](https://github.com/vinaayakh/designer/releases) page has six archives and `checksums.txt`.
- Install it fresh with each installer:
  ```sh
  curl -fsSL https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.sh | sh
  designer --version
  ```
  ```powershell
  irm https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.ps1 | iex
  designer --version
  ```
- `gh attestation verify designer_<version>_linux_amd64.tar.gz --repo vinaayakh/designer` succeeds.
- Open a Project and run one Assistant task end to end.

## Repository settings (once)

- **Settings → Actions → General:** allow GitHub Actions; workflow permissions "Read repository contents" (the workflows ask for more where needed).
- **Settings → Code security:** enable private vulnerability reporting (SECURITY.md links to it), Dependabot alerts and secret scanning.
- **Settings → General → Features:** enable Discussions (the issue chooser links to it).
- **Branch protection on `main`:** require the CI checks to pass.
- **Tag protection:** restrict who can create `v*` tags.
