## BBS Migrations

### Schema versions

BBS database migrations are always run at process startup.

Every migration has a version number. Migrations are sorted on this version
number and then applied in sequence. After each migration is run, the version
number for that migration is stored in the database.  On future migration runs,
migrations with a version number lower or equal to the current stored version
will not be run again.

### Migration Requirements

Migrations must be idempotent, meaning that when a migration of version N is
applied against schema version N, no change occurs. There is no expectation of
or requirement for migrations to be interchangeable, meaning migrations are not
expected to run against any schema versions other than N or N - 1 (where N is
the schema version defined by the current migration).  Additionally, rollbacks
and "down" migrations are not supported.

### Writing a migration

Use `scripts/make_migration.sh <name>` to create a new migration. `<name>`
should be a short description of what the migration does, and should be
provided in `snake_case`.

This will create test and implementation files for your migration in
`db/migrations`. Look for `TODO` comments in the test file to get started.

Note that the test file includes a test for migration idempotency.

