# Publish to Maven Central

Published artifacts (version **0.1.0**):

| Artifact | Coordinates |
|----------|-------------|
| Protocol | `io.github.andriyg76.simple-chat:protocol:0.1.0` |
| Core | `io.github.andriyg76.simple-chat:core:0.1.0` |
| Server | `io.github.andriyg76.simple-chat:server:0.1.0` |

Not published: `test-server`, `client`.

## One-time setup

1. Register at [Central Portal](https://central.sonatype.com/) and claim namespace **`io.github.andriyg76`** (GitHub verification for `io.github.andriyg76.simple-chat` artifacts).
2. Create a user token in the portal.
3. Add to `~/.m2/settings.xml`:

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

4. GPG: publish a signing key and register it in your Sonatype profile.

```bash
gpg --full-generate-key
gpg --keyserver keyserver.ubuntu.com --send-keys <KEY_ID>
```

## Deploy command

From `java/`:

```bash
mvn clean deploy -Prelease -pl protocol,core,server -am
```

Or deploy the whole reactor (test-server and client are skipped):

```bash
mvn clean deploy -Prelease
```

Requires network, valid `central` credentials, and GPG signing on the `verify` phase.
