package models

type Signal struct {
	Market      string
	Position    string
	EntryPoints []string
	Targets     []string
	StopLoss    string
	Leverage    string
}
