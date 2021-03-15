package migrations

import "sort"

type MigrationStepsProvider struct {
	migrationSteps []MigrationStep
}

func NewMigrationStepsProvider(migrationSteps []MigrationStep) MigrationStepsProvider {
	sort.Slice(migrationSteps, func(i, j int) bool {
		return migrationSteps[i].SequenceID() < migrationSteps[j].SequenceID()
	})

	return MigrationStepsProvider{migrationSteps: migrationSteps}
}

func (p MigrationStepsProvider) Provide() []MigrationStep {
	return p.migrationSteps
}

func (p MigrationStepsProvider) GetLatestMigrationIndex() int {
	migrationSteps := p.Provide()

	return migrationSteps[len(migrationSteps)-1].SequenceID()
}
