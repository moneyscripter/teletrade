package bot

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/moneyscripter/teletrade/channels"
	"github.com/moneyscripter/teletrade/exchanges"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var updateChan = make(chan *models.Update, 1000)

// Mock Database for Subscribed Users
var userInfo = make(map[int64]*Info)

func ActiveUsers() map[int64]*Info {
	return userInfo
}

var botMessageIDs = make(map[int64][]int)

// Track the message sent by the bot
func trackBotMessage(chatID int64, messageID int) {
	botMessageIDs[chatID] = append(botMessageIDs[chatID], messageID)
}

type Info struct {
	mutex            *sync.RWMutex
	IsRunning        bool
	ChannelIDs       []string
	Exchange         string
	APIKey           string
	WaitingApiKey    bool
	SecretKey        string
	WaitingSecretKey bool
}

func (i *Info) AddChannelID(channelID string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	for _, channel := range i.ChannelIDs {
		if channel == channelID {
			return
		}
	}
	i.ChannelIDs = append(i.ChannelIDs, channelID)
}

func (i *Info) RemoveChannelID(channelID string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	for index := range i.ChannelIDs {
		if i.ChannelIDs[index] == channelID {
			i.ChannelIDs = append(i.ChannelIDs[:index], i.ChannelIDs[index+1:]...)
			break
		}
	}
}

func (i *Info) UpdateExchange(exchange string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.Exchange = exchange
}

func (i *Info) UpdateApiKey(apiKey string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.APIKey = apiKey
}

func (i *Info) ApiKeyWaiting(b bool) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.WaitingApiKey = b
}

func (i *Info) UpdateSecret(secretKey string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.SecretKey = secretKey
}

func (i *Info) SecretKeyWaiting(b bool) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.WaitingSecretKey = b
}

func (i *Info) Start() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.IsRunning = true
}

func (i *Info) Stop() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.IsRunning = false
}

// Run Send any text message to the bot after the bot has been started
func Run(token string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(userInputHandler),
	}

	b, err := bot.New(token, opts...)
	if nil != err {
		return err
	}

	updateBot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Separate goroutine to handle channel updates and forwarding
	go func() {
		fmt.Println(updateBot.Send(tgbotapi.NewMessage(-1002239676669, "Bot has been started!")))

		for update := range updateChan {
			fmt.Println(updateBot.Send(tgbotapi.NewMessage(-1002239676669, fmt.Sprintf("%+v", update))))
		}
	}()

	b.RegisterHandler(bot.HandlerTypeMessageText, "/hello", bot.MatchTypeExact, helloHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, homeHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, callbackQueryHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeExact, userInputHandler)

	b.Start(ctx)

	return nil
}

// This function handles channel updates
func handleUpdate(b *bot.Bot, update tgbotapi.Update, destinationID int64) {
	if update.Message != nil {
		//sourceChannelID := "@source_channel"

		// Check if the message is from the source channel
		//if update.Message.Chat.ID == sourceChannelID {
		// Forward the message to the destination channel
		b.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID: destinationID,
			Text:   fmt.Sprintf("%d: %s", update.Message.Chat.ID, update.Message.Text),
		})
		//}
	}
}

func helloHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "Hello, *" + bot.EscapeMarkdown(update.Message.From.FirstName) + "*",
		ParseMode: models.ParseModeMarkdown,
	})
}

func channelSelection(ctx context.Context, b *bot.Bot, chatID int64) {
	// Create an inline keyboard for the available channels
	var buttons [][]models.InlineKeyboardButton
	for channelID, _ := range channels.AvailableChannels {
		var row []models.InlineKeyboardButton
		channelsButton := models.InlineKeyboardButton{
			Text:         channelID,
			CallbackData: "channel_" + channelID,
		}
		row = append(row, channelsButton)
		redirectButton := models.InlineKeyboardButton{
			Text: "Redirect",
			URL:  channels.AvailableChannels[channelID],
		}
		row = append(row, redirectButton)
		buttons = append(buttons, row)
	}

	backButton := models.InlineKeyboardButton{
		Text:         "Back",
		CallbackData: "channels",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{backButton})

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Please select a channel:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func exchangeSelection(ctx context.Context, b *bot.Bot, chatID int64) {
	// Create an inline keyboard for the available exchanges
	var buttons [][]models.InlineKeyboardButton
	for exchange, _ := range exchanges.AvailableExchanges {
		var row []models.InlineKeyboardButton
		button := models.InlineKeyboardButton{
			Text:         exchange,
			CallbackData: "exchange_" + exchange,
		}
		row = append(row, button)
		redirectButton := models.InlineKeyboardButton{
			Text: "Redirect",
			URL:  exchanges.AvailableExchanges[exchange],
		}
		row = append(row, redirectButton)
		buttons = append(buttons, row)
	}

	backButton := models.InlineKeyboardButton{
		Text:         "Back",
		CallbackData: "exchange",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{backButton})

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Please select an exchange:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func homeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userState(ctx, b, update.Message.Chat.ID)
}

func userState(ctx context.Context, b *bot.Bot, chatID int64) {
	info, ok := subscription(ctx, b, chatID)
	if !ok {
		return
	}

	var buttons [][]models.InlineKeyboardButton

	channelsButton := models.InlineKeyboardButton{
		Text:         fmt.Sprintf("Channels (%d)", len(info.ChannelIDs)),
		CallbackData: "channels",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{channelsButton})

	exchangeButtonText := "Exchange (Not Selected)"
	if info.Exchange != "" {
		exchangeButtonText = fmt.Sprintf("Exchange (%s)", info.Exchange)
	}
	exchangeButton := models.InlineKeyboardButton{
		Text:         exchangeButtonText,
		CallbackData: "exchange",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{exchangeButton})

	if info.IsRunning {
		startButton := models.InlineKeyboardButton{
			Text:         "Stop",
			CallbackData: "stop",
		}
		buttons = append(buttons, []models.InlineKeyboardButton{startButton})
	} else {
		stopButton := models.InlineKeyboardButton{
			Text:         "Start",
			CallbackData: "start",
		}
		buttons = append(buttons, []models.InlineKeyboardButton{stopButton})
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Your account is subscribed:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func channelState(ctx context.Context, b *bot.Bot, chatID int64) {
	buttons := [][]models.InlineKeyboardButton{}
	for _, channelID := range userInfo[chatID].ChannelIDs {
		var row []models.InlineKeyboardButton
		channelsButton := models.InlineKeyboardButton{
			Text:         channelID,
			CallbackData: "channels",
		}
		row = append(row, channelsButton)
		redirectButton := models.InlineKeyboardButton{
			Text: "Redirect",
			URL:  channels.AvailableChannels[channelID],
		}
		row = append(row, redirectButton)
		removeButton := models.InlineKeyboardButton{
			Text:         "❌",
			CallbackData: "remove_channel_" + channelID,
		}
		row = append(row, removeButton)
		buttons = append(buttons, row)
	}
	channelsButton := models.InlineKeyboardButton{
		Text:         "Add Channel",
		CallbackData: "select_channel",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{channelsButton})

	backButton := models.InlineKeyboardButton{
		Text:         "Back",
		CallbackData: "home",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{backButton})

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Channels:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
}

func exchangeState(ctx context.Context, b *bot.Bot, chatID int64) {
	buttons := [][]models.InlineKeyboardButton{}
	row1 := []models.InlineKeyboardButton{}
	flag := true
	if userInfo[chatID].Exchange == "" {
		userInfo[chatID].Exchange = "Not Selected"
		flag = false
	}
	exchangeButton := models.InlineKeyboardButton{
		Text:         userInfo[chatID].Exchange,
		CallbackData: "set_exchange",
	}
	row1 = append(row1, exchangeButton)
	if flag {
		redirectButton := models.InlineKeyboardButton{
			Text: "Redirect",
			URL:  exchanges.AvailableExchanges[userInfo[chatID].Exchange],
		}
		row1 = append(row1, redirectButton)
	}
	buttons = append(buttons, row1)

	row2 := []models.InlineKeyboardButton{}
	apiKey := userInfo[chatID].APIKey
	if apiKey == "" {
		apiKey = "API KEY: EMPTY"
	}
	apiKeyButton := models.InlineKeyboardButton{
		Text:         apiKey,
		CallbackData: "set_api_key",
	}
	row2 = append(row2, apiKeyButton)
	secretKey := userInfo[chatID].SecretKey
	if secretKey == "" {
		secretKey = "API KEY: EMPTY"
	}
	secretKeyButton := models.InlineKeyboardButton{
		Text:         secretKey,
		CallbackData: "set_secret_key",
	}
	row2 = append(row2, secretKeyButton)
	buttons = append(buttons, row2)

	backButton := models.InlineKeyboardButton{
		Text:         "Back",
		CallbackData: "home",
	}
	buttons = append(buttons, []models.InlineKeyboardButton{backButton})

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Exchange:",
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
	fmt.Println(err)
}

func callbackQueryHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	query := update.CallbackQuery
	data := query.Data
	chatID := query.Message.Message.Chat.ID

	_, ok := subscription(ctx, b, chatID)
	if !ok && data != "subscribe" {
		return
	}

	if data == "" {
		return
	}

	switch data {
	case "home":
		userState(ctx, b, chatID)
	case "subscribe":
		userInfo[chatID] = &Info{
			mutex:            &sync.RWMutex{},
			IsRunning:        false,
			ChannelIDs:       nil,
			Exchange:         "",
			APIKey:           "",
			WaitingApiKey:    false,
			SecretKey:        "",
			WaitingSecretKey: false,
		}
		userState(ctx, b, chatID)
	case "channels":
		channelState(ctx, b, chatID)
	case "select_channel":
		channelSelection(ctx, b, chatID)
	case "exchange":
		exchangeState(ctx, b, chatID)
	case "set_exchange":
		exchangeSelection(ctx, b, chatID)
	case "set_api_key":
		userInfo[chatID].ApiKeyWaiting(true)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please enter your access key:",
		})
	case "set_secret_key":
		userInfo[chatID].SecretKeyWaiting(true)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please enter your secret:",
		})
	case "start":
		userInfo[chatID].Start()
	case "stop":
		userInfo[chatID].Stop()
	default:
		if strings.HasPrefix(data, "channel_") {
			selectedChannel := strings.TrimPrefix(data, "channel_")
			userInfo[chatID].AddChannelID(selectedChannel)

			channelState(ctx, b, chatID)
		}
		if strings.HasPrefix(data, "remove_channel_") {
			selectedChannel := strings.TrimPrefix(data, "remove_channel_")
			userInfo[chatID].RemoveChannelID(selectedChannel)

			channelState(ctx, b, chatID)
		}

		if strings.HasPrefix(data, "exchange_") {
			selectedExchange := strings.TrimPrefix(data, "exchange_")
			userInfo[chatID].UpdateExchange(selectedExchange)

			exchangeState(ctx, b, chatID)
		}
	}

	// Acknowledge the callback query
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	})
}

func userInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	//if update.ChannelPost != nil {
	//	b.SendMessage(ctx, &bot.SendMessageParams{
	//		ChatID: -1002239676669,
	//		Text:   fmt.Sprintf("%d: %s", update.ChannelPost.Chat.ID, update.ChannelPost.Text),
	//	})
	//}
	updateChan <- update
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	message := update.Message.Text

	_, ok := subscription(ctx, b, chatID)
	if !ok {
		return
	}

	// Check if access key is already provided
	if userInfo[chatID].WaitingApiKey {
		userInfo[chatID].UpdateApiKey(message)
		userInfo[chatID].ApiKeyWaiting(false)
		exchangeState(ctx, b, chatID)
	} else if userInfo[chatID].WaitingSecretKey {
		userInfo[chatID].UpdateSecret(message)
		userInfo[chatID].SecretKeyWaiting(false)
		exchangeState(ctx, b, chatID)
	} else {
		userState(ctx, b, chatID)
	}
}

func subscription(ctx context.Context, b *bot.Bot, chatID int64) (*Info, bool) {
	info, ok := userInfo[chatID]
	if !ok {
		var buttons [][]models.InlineKeyboardButton
		button := models.InlineKeyboardButton{
			Text:         "Subscribe",
			CallbackData: "subscribe",
		}
		buttons = append(buttons, []models.InlineKeyboardButton{button})
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Your accounts is not subscribed yet:",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})
		return nil, false
	}
	return info, true
}
