# Releases and branches

## Development versions

Java reactor (`java/pom.xml` and modules) is at the next patch snapshot **`X.Y.P-SNAPSHOT`** between releases;
releasing it publishes **`X.Y.P`** and moves the branch to **`X.Y.(P+1)-SNAPSHOT`**. When a `release/X.Y` branch
is cut, move `main` to **`X.(Y+1).0-SNAPSHOT`** so the two branches never publish the same snapshot.

## Branches

| Branch | Purpose |
|--------|---------|
| **`main`** | Integration; releases its current snapshot. |
| **`release/X.Y`** | Optional maintenance line (**X.Y.P**) once `main` has moved to the next minor. |

## Publish (GitHub Actions)

**Actions → Release to Maven Central → Run workflow**

| Input | Default | Meaning |
|-------|---------|---------|
| **Release version** | empty | **`X.Y.P`**; empty releases the branch's current `X.Y.P-SNAPSHOT` |
| **Branch** | `main` | `main` or `release/X.Y` |

`protocol`, `core` and `server` share one version and one tag, so they are always released together.

The workflow:

1. Sets all reactor poms to **`X.Y.P`**, commits, tags **`vX.Y.P`** and pushes the commit and tag
   atomically **before** publishing (a moved branch fails the run with nothing published)
2. **`mvn deploy -Prelease`** for `protocol,core,server` (skipped when the version is already on Central)
3. Sets the branch to the next patch snapshot **`X.Y.(P+1)-SNAPSHOT`**
4. Creates a GitHub release for the tag with the commits since the previous `v*` tag

A failed run can simply be re-run with the same inputs: an existing tag is reused, an already published
version is not deployed again, and an existing GitHub release is kept.

Secrets: see [java/MAVEN_CENTRAL.md](java/MAVEN_CENTRAL.md).

To keep patching `0.1` while `main` moves on, cut a maintenance branch from the latest tag and move `main`
to the next minor:

```bash
git fetch origin
git checkout -b release/0.1 v0.1.1
(cd java && bash scripts/set-reactor-version.sh 0.1.2-SNAPSHOT)
git commit -am "Prepare 0.1.2-SNAPSHOT development."
git push -u origin release/0.1
# then on main: set-reactor-version.sh 0.2.0-SNAPSHOT
```

Patch releases from it: run the workflow on **`release/0.1`**.

Forward-port fixes from `release/X.Y` to `main` when needed.

## Snapshots

Every push to **`main`** or **`release/**`** that touches `java/` (including merged PRs) runs
[`.github/workflows/snapshot.yml`](.github/workflows/snapshot.yml), which deploys `protocol`, `core` and
`server` at the current **`X.Y.P-SNAPSHOT`** to the Central snapshot repository. Consumers add:

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
