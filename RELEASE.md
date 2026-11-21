# Releases and branches

## Development versions

Java reactor (`java/pom.xml` and modules) uses **`X.Y-SNAPSHOT`** on **`main`** and **`release/X.Y`** between releases.

## Branches

| Branch | Purpose |
|--------|---------|
| **`main`** | Integration; next **X.Y.0** from snapshot. |
| **`release/X.Y`** | Patch line (**X.Y.*** bugfixes). |

## Publish (GitHub Actions)

**Actions → Release to Maven Central → Run workflow**

| Input | Default | Meaning |
|-------|---------|---------|
| **Release version** | `0.1.0` | **`X.Y.P`** — sets poms, tag **`vX.Y.P`**, deploy |
| **Module(s)** | `protocol,core,server` | `-pl` for Central (or single artifact) |
| **Branch** | `main` | `main` or `release/X.Y` |

The workflow:

1. Sets all reactor poms to **`X.Y.P`**
2. Commits, tags **`vX.Y.P`**
3. **`mvn deploy -Prelease`** for the chosen module(s)
4. Sets poms back to **`X.Y-SNAPSHOT`** (major.minor from the release)
5. Commits and pushes the branch **and** the tag

Secrets: see [java/MAVEN_CENTRAL.md](java/MAVEN_CENTRAL.md).

After the first **`0.1.0`** release from `main`, create the maintenance branch:

```bash
git fetch origin
git checkout -b release/0.1 v0.1.0
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
