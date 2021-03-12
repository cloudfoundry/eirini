package migrations

type ProductionMigrationStepsProvider struct {
	cpuRequestSetter CPURequestSetter
}

func NewProductionMigrationStepsProvider(cpuRequestSetter CPURequestSetter) ProductionMigrationStepsProvider {
	return ProductionMigrationStepsProvider{cpuRequestSetter: cpuRequestSetter}
}

func (p ProductionMigrationStepsProvider) Provide() []MigrationStep {
	return []MigrationStep{
		NewAdjustCPURequest(p.cpuRequestSetter),
	}
}

func (p ProductionMigrationStepsProvider) GetLatestMigrationIndex() int {
	max := -1

	for _, s := range p.Provide() {
		seq := s.SequenceID()
		if seq > max {
			max = seq
		}
	}

	return max
}
