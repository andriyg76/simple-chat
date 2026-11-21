# Releases and branches

## Development versions

Java reactor (`java/pom.xml` and modules) is always a snapshot between releases, and each branch owns its own
snapshot so the snapshot repository never mixes them: **`main`** is at the next minor (**`X.(Y+1)-SNAPSHOT`**
once **`X.Y.0`** is out), **`release/X.Y`** is at **`X.Y-SNAPSHOT`**.

## Branches

| Branch | Purpose |
|--------|---------|
| **`main`** | Integration; releases **X.Y.0** only. |
| **`release/X.Y`** | Patch line (**X.Y.P**, P ≥ 1). |

## Publish (GitHub Actions)

**Actions → Release to Maven Central → Run workflow**

| Input | Default | Meaning |
|-------|---------|---------|
| **Release version** | `0.1.0` | **`X.Y.P`** — `X.Y.0` from `main`, `X.Y.P` from `release/X.Y` |
| **Branch** | `main` | `main` or `release/X.Y` |

`protocol`, `core` and `server` share one version and one tag, so they are always released together.

The workflow:

1. Sets all reactor poms to **`X.Y.P`**, commits, tags **`vX.Y.P`** and pushes the commit and tag
   atomically **before** publishing (a moved branch fails the run with nothing published)
2. **`mvn deploy -Prelease`** for `protocol,core,server` (skipped when the version is already on Central)
3. Sets the branch to its next snapshot (`main` → `X.(Y+1)-SNAPSHOT`, `release/X.Y` → `X.Y-SNAPSHOT`)
4. Creates a GitHub release for the tag with the commits since the previous `v*` tag

A failed run can simply be re-run with the same inputs: an existing tag is reused, an already published
version is not deployed again, and an existing GitHub release is kept.

Secrets: see [java/MAVEN_CENTRAL.md](java/MAVEN_CENTRAL.md).

Create a maintenance branch from a release tag and move it to its snapshot:

```bash
git fetch origin
git checkout -b release/0.1 v0.1.0
(cd java && bash scripts/set-reactor-version.sh 0.1-SNAPSHOT)
git commit -am "Prepare 0.1-SNAPSHOT development."
git push -u origin release/0.1
```

Patch releases: run the workflow on **`release/0.1`** with e.g. **`0.1.1`**, **`0.1.2`**, …

Forward-port fixes from `release/X.Y` to `main` when needed.

## Snapshots

Every push to **`main`** or **`release/**`** that touches `java/` (including merged PRs) runs
[`.github/workflows/snapshot.yml`](.github/workflows/snapshot.yml), which deploys `protocol`, `core` and
`server` at the current **`X.Y-SNAPSHOT`** to the Central snapshot repository. Consumers add:

```xml
<repository>
  <id>central-snapshots</id>
  <url>https://central.sonatype.com/repository/maven-snapshots/</url>
  <releases><enabled>false</enabled></releases>
  <snapshots><enabled>true</enabled></snapshots>
</repository>
```

## CI

Push / PR to **`main`** or **`release/**`**: [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (tests only).
