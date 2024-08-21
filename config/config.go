package config

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
)

var AppConfig *config // global app config

type config struct {
	TelegramClient telegramClient `mapstructure:"telegram_client"`
	TelegramBot    telegramBot    `mapstructure:"telegram_bot"`
}

type telegramClient struct {
	Phone   string `mapstructure:"phone"`
	AppID   int    `mapstructure:"app_id"`
	AppHash string `mapstructure:"app_hash"`
}

type telegramBot struct {
	Token string `mapstructure:"token"`
}

func LoadConfig(path string) {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("json")   // REQUIRED if the config file does not have the extension in the name

	if path == "" {
		viper.AddConfigPath("./app/config") // path to look for the config file in
		viper.AddConfigPath("./config")     // path to look for the config file in
		viper.AddConfigPath(".")            // optionally look for config in the working directory
	} else {
		viper.SetConfigFile(path)
	}

	viper.SetEnvPrefix("TeleTrade")
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.AutomaticEnv() // read in environment variables that match

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	AppConfig = &config{}
	if err = viper.Unmarshal(&AppConfig); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
}
