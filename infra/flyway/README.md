# Flyway Config

Create per-environment Flyway config files in this directory.

Example: `infra/flyway/dev.conf`

```conf
flyway.url=jdbc:postgresql://localhost:5432/accountlink
flyway.user=accountlink
flyway.password=accountlink
flyway.locations=filesystem:./migrations
```

`just flyway-migrate <env>` expects `infra/flyway/<env>.conf`.
