package main

import (
	"context"
	"fmt"
	"github.com/moneyscripter/teletrade/channels/CryptoTrade066"
	"github.com/moneyscripter/teletrade/config"
	"github.com/moneyscripter/teletrade/exchanges"
	"github.com/moneyscripter/teletrade/exchanges/coinex"
	"github.com/moneyscripter/teletrade/telegram_engine/bot"
	"github.com/moneyscripter/teletrade/telegram_engine/client"
	"os"
	"os/signal"
	"time"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	config.LoadConfig(configPath)

	// Telegram Bot
	bot.Run(config.AppConfig.TelegramBot.Token)
	var receivingChannels []client.ReceivingChannel
	cryptoTrade006Channel, cryptoTrade006ChannelID := CryptoTrade066.NewCryptoTrade0066()
	receivingChannels = append(receivingChannels, client.ReceivingChannel{
		Chan:      make(chan string, 1000),
		ChannelID: cryptoTrade006ChannelID,
		Parser:    cryptoTrade006Channel,
	})

	phone := config.AppConfig.TelegramClient.Phone
	appID := config.AppConfig.TelegramClient.AppID
	appHash := config.AppConfig.TelegramClient.AppHash

	telegramEngine := client.Engine{
		Phone:             phone,
		AppID:             appID,
		AppHash:           appHash,
		ReceivingChannels: receivingChannels,
	}

	ct := context.Background()
	ctx, cancelFunc := context.WithCancel(ct)
	go func() {
		for {
			err := telegramEngine.Run(ctx)
			if err != nil {
				fmt.Printf("error: %v", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	var contextMap map[int64][]context.CancelFunc
	var exchangeMap map[int64]exchanges.Exchanges
	for _, receivingChannel := range receivingChannels {
		go func(receivingChannel client.ReceivingChannel) {
			for {
				select {
				case msg := <-receivingChannel.Chan:
					fmt.Println("Received Signal on channel id: ", receivingChannel.ChannelID)
					sig, ok := receivingChannel.Parser.ParsSignal(msg)
					if !ok {
						continue
					}

					for chatID, exchange := range exchangeMap {
						fmt.Println("Signal is shipped to chat id: ", chatID)
						newContext, cf := context.WithCancel(ctx)
						err := exchange.Execute(newContext, sig)
						if err != nil {
							fmt.Println(err)
						}
						if _, exists := contextMap[chatID]; !exists {
							contextMap[chatID] = []context.CancelFunc{cf}
						} else {
							contextMap[chatID] = append(contextMap[chatID], cf)
						}
					}
				}
			}
		}(receivingChannel)
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			activeUsers := bot.ActiveUsers()
			for chatID, info := range activeUsers {
				if info.IsRunning {
					_, exists := exchangeMap[chatID]
					if !exists {
						if info.APIKey == "" || info.SecretKey == "" {
							continue
						}
						switch info.Exchange {
						case "Coinex":
							coinexEngine := coinex.NewCoinexEngine(info.APIKey, info.SecretKey)
							exchangeMap[chatID] = coinexEngine
						default:
						}
					} else {
						delete(exchangeMap, chatID)
						if value, ok := contextMap[chatID]; ok {
							for _, v := range value {
								v()
							}
							delete(contextMap, chatID)
						}
					}
				}
			}
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server with a timeout of X seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	cancelFunc()
}
