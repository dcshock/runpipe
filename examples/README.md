# Examples

Example pipelines that use runpipe with optional DB persistence.

## with-db

A pipeline that uses **stdlib stages** (Validate, Tap, ParkStageAfter, MapSlice), **DBObserver**, **ParkedRunStore**, and **Resumer** against Postgres.

- **Run with a real DB:** set `DATABASE_URL` (e.g. `postgres://user:pass@localhost:5432/dbname`), apply the migration in `with-db/migration.sql`, then:
  ```bash
  cd examples/with-db && go run .
  ```
- **Test:** uses [testcontainers](https://golang.testcontainers.org/) to start Postgres; requires Docker.
  ```bash
  cd examples/with-db && go test -v .
  ```

The pipeline validates input `[]string`, taps, parks for 2 seconds, then (on resume) coerces JSON-unmarshaled input to `[]string` and maps to uppercase.
