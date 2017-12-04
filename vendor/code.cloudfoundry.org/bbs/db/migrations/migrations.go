package migrations

import "code.cloudfoundry.org/bbs/migration"

var Migrations = []migration.Migration{}

func AppendMigration(migration migration.Migration) {
	for _, m := range Migrations {
		if m.Version() == migration.Version() {
			panic("cannot have two migrations with the same version")
		}
	}

	Migrations = append(Migrations, migration)
}
