# Publish to Maven Central

Published artifacts (example **0.1.0**):

| Artifact | Coordinates |
|----------|-------------|
| Protocol | `io.github.andriyg76.simple-chat:protocol:0.1.0` |
| Core | `io.github.andriyg76.simple-chat:core:0.1.0` |
| Server | `io.github.andriyg76.simple-chat:server:0.1.0` |

Not published: `test-server`, `client`.

Between releases the reactor version is **`X.Y-SNAPSHOT`** (see [RELEASE.md](../RELEASE.md)).

## One-time setup

1. Register at [Central Portal](https://central.sonatype.com/) and claim namespace **`io.github.andriyg76`**.
2. Create a user token in the portal.
3. GitHub repo secrets: `MAVEN_CENTRAL_USERNAME`, `MAVEN_CENTRAL_PASSWORD`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`.
4. GPG key registered in Sonatype profile.

Local `~/.m2/settings.xml` (optional manual deploy):

```xml
<settings>
  <servers>
    <server>
      <id>central</id>
      <username><!-- token username --></username>
      <password><!-- token password --></password>
    </server>
  </servers>
</settings>
```

## Release workflow

[`.github/workflows/release.yml`](../.github/workflows/release.yml) — **workflow_dispatch**:

- **Release version** (default `0.1.0`) → `vX.Y.P` tag and release poms
- **Branch** (default `main`)

After deploy, the branch moves to its next snapshot (see [RELEASE.md](../RELEASE.md)).

## Local version bump (without CI)

From `java/`:

```bash
bash scripts/set-reactor-version.sh 0.1.2   # release
bash scripts/set-reactor-version.sh 0.1-SNAPSHOT
```

Manual deploy at a release version:

```bash
mvn clean deploy -Prelease -pl protocol,core,server -am
```
