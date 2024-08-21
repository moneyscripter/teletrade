package channels

import "github.com/moneyscripter/teletrade/models"

type Channels interface {
	ParsSignal(message string) (models.Signal, bool)
}

var AvailableChannels = map[string]string{
	"CryptoTrade066": "https://t.me/CryptoTrade066",
}
