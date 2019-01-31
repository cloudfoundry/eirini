package db

//go:generate counterfeiter . DB

type DB interface {
	DomainDB
	EncryptionDB
	EvacuationDB
	LRPDB
	TaskDB
	VersionDB
	SuspectDB
}
