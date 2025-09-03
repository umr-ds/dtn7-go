package bpv7

type RECNodeType uint8

const (
	NTypeBroker    RECNodeType = 1
	NTypeExecutor  RECNodeType = 2
	NTypeDataStore RECNodeType = 3
	NTypeClient    RECNodeType = 4
)
