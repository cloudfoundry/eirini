package migrations

import (
	"sort"

	"code.cloudfoundry.org/eirini/k8s/client"
)

func CreateMigrationStepsProvider(stSetClient *client.StatefulSet, pdbClient *client.PodDisruptionBudget, workloadsNamespace string) MigrationProvider {
	migrationSteps := []MigrationStep{
		NewAdjustCPURequest(stSetClient),
		NewAdoptPDB(pdbClient),
	}

	return NewMigrationStepsProvider(migrationSteps)
}

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
