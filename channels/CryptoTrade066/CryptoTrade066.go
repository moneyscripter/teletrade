package CryptoTrade066

import (
	"github.com/moneyscripter/teletrade/channels"
	"github.com/moneyscripter/teletrade/models"
	"strings"
)

type cryptoTrade0066 struct {
}

// NewCryptoTrade0066 is a constructor for cryptoTrade0066
func NewCryptoTrade0066() (channels.Channels, int64) {
	return cryptoTrade0066{}, 1261856999
}

func (c cryptoTrade0066) ParsSignal(message string) (models.Signal, bool) {
	mustFound := []string{
		"نام",
		"نوع پوزیشن",
		"نقطه ورود",
		"تارگت",
		"حدضرر",
		"اهرم",
	}

	signal := models.Signal{}
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if strings.Contains(line, "سیگنال") {
			break
		}
	}
	for i, key := range mustFound {
		flag := false
		for _, line := range lines {
			if strings.Contains(line, key) {
				flag = true
				if i == 0 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.Market = parts[1]
				}
				if i == 1 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.Position = parts[1]
				}
				if i == 2 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.EntryPoints = append(signal.EntryPoints, parts[1])
				}
				if i == 3 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.Targets = append(signal.Targets, parts[1])
				}
				if i == 4 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.StopLoss = parts[1]
				}
				if i == 5 {
					parts := strings.Split(line, ":")
					if len(parts) != 2 {
						flag = false
						break
					}
					parts[1] = strings.ReplaceAll(parts[1], " ", "")
					parts[1] = strings.ReplaceAll(parts[1], "-", "")
					parts[1] = strings.ReplaceAll(parts[1], "/", "")
					signal.Leverage = parts[1]
				}
			}
		}
		if !flag {
			return models.Signal{}, false
		}
	}
	return signal, true
}
