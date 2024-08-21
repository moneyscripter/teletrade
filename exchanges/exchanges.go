package exchanges

import (
	"context"
	"github.com/moneyscripter/teletrade/models"
)

type Exchanges interface {
	Execute(ctx context.Context, signal models.Signal) error
}

var AvailableExchanges = map[string]string{
	"Coinex": "https://www.coinex.com",
}
