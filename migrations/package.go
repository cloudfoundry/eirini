package migrations

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	AdjustCPUResourceSequenceID = 1
	AdoptPDBSequenceID          = 2
	AdoptStSetSecretSequenceID  = 3
	AdoptJobSecretSequenceID    = 4
)
