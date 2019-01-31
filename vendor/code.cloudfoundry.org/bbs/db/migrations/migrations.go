package migrations

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"code.cloudfoundry.org/bbs/migration"
)

var migrationsRegistry = migration.Migrations{}

func appendMigration(migrationTemplate migration.Migration) {
	migrationsRegistry = append(migrationsRegistry, migrationTemplate)
}

func migrationString(m migration.Migration) string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return strconv.FormatInt(m.Version(), 10)
	}
	return strings.Split(filepath.Base(filename), ".")[0]
}

func AllMigrations() migration.Migrations {
	migs := make(migration.Migrations, len(migrationsRegistry))
	for i, mig := range migrationsRegistry {
		rt := reflect.TypeOf(mig)
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		migs[i] = reflect.New(rt).Interface().(migration.Migration)
	}
	return migs
}
