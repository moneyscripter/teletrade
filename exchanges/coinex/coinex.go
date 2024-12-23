package coinex

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/moneyscripter/teletrade/exchanges"
	"github.com/moneyscripter/teletrade/models"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	coinexBaseURL = "https://api.coinex.com"
)

type OrderRequest struct {
	Market string `json:"market"`
	Type   string `json:"type"` // "limit" or "market"
	Side   string `json:"side"` // "buy" or "sell"
	Amount string `json:"amount"`
	Price  string `json:"price,omitempty"`
}

type engine struct {
	ApiKey    string
	SecretKey string
}

func NewCoinexEngine(apiKey, secretKey string) exchanges.Exchanges {
	return &engine{
		ApiKey:    apiKey,
		SecretKey: secretKey,
	}
}

//func main() {
//	// Example usage
//	signal := models.Signal{
//		Position:    "sell",
//		EntryPoints: []string{"28.92"},                      // Example entry point
//		Targets:     []string{"27.988", "27.706", "27.340"}, // Example targets
//		StopLoss:    "30.140",                               // Example stop loss
//		Leverage:    "10",                                   // Example leverage
//	}
//
//	market := "ORDIUSDT" // Example market
//	amount := "0.5"      // Example amount
//
//	instance := engine{
//		ApiKey:    "111532590CC3499C8C5931D388A726D5",
//		SecretKey: "2882C8E506CFC5D3617FD41D5CC66F881906B4200A4A9BBF",
//	}
//	err := instance.executeStrategy(signal, market, amount)
//	if err != nil {
//		fmt.Printf("Error executing strategy: %v\n", err)
//	}
//}

func (c *engine) Execute(ctx context.Context, signal models.Signal) error {
	leverage, err := strconv.Atoi(signal.Leverage)
	if err != nil {
		return err
	}
	positionAmount, err := c.calPositionAmount(signal.Market, leverage)
	if err != nil {
		return err
	}
	// Step 1: Place the initial order (using the first entry point)
	entryPrice := signal.EntryPoints[0]
	_, err = c.placeStopOrder(signal.Position, signal.Market, positionAmount, entryPrice)
	if err != nil {
		return fmt.Errorf("failed to place initial order: %v", err)
	}
	fmt.Printf("Order placed [ market: %s, position: %s, entry price: %s ]\n", signal.Market, signal.Position, entryPrice)

	err = c.waitPositionOpened(signal.Market)
	if err != nil {
		return fmt.Errorf("waiting for opened position failed: %v", err)
	}

	// Step 2: Place stop-loss and take profit orders
	//err = c.placeTakeProfitAndStopLossOrders(signal, market, amount)
	//if err != nil {
	//	return fmt.Errorf("failed to place TP/SL orders: %v", err)
	//}
	fmt.Printf("Position entered - market: %s, position: %s, entry price: %s\n", signal.Market, signal.Position, entryPrice)
	err = c.placeTP(signal.Market, signal.Targets[0])
	if err != nil {
		return fmt.Errorf("failed to place TP: %v", err)
	}
	err = c.placeSL(signal.Market, signal.StopLoss)
	if err != nil {
		return fmt.Errorf("failed to place SL: %v", err)
	}

	// Step 3: Monitor the position
	err = c.waitPositionClosed(signal.Market)
	if err != nil {
		return fmt.Errorf("failed to monitor position: %v", err)
	}
	fmt.Printf("Position closed - market: %s, position: %s, entry price: %s\n", signal.Market, signal.Position, entryPrice)

	return nil
}

func (c *engine) placeTakeProfitAndStopLossOrders(signal models.Signal, market, amount string) error {
	// Step 2a: Place the stop-loss order (opposite side of the initial position)
	oppositeSide := "sell"
	if signal.Position == "sell" {
		oppositeSide = "buy"
	}
	_, err := c.placeOrder(oppositeSide, market, amount, signal.StopLoss)
	if err != nil {
		return fmt.Errorf("failed to place stop-loss order: %v", err)
	}

	// Step 2b: Place take profit orders
	m, err := strconv.Atoi(amount)
	if err != nil {
		return fmt.Errorf("failed to convert amount to int: %v", err)
	}

	x := 0
	eachAmount := m / len(signal.Targets)
	if eachAmount*len(signal.Targets) != m {
		x = m - eachAmount*len(signal.Targets)
	}
	for i, target := range signal.Targets {
		newAmount := strconv.Itoa(eachAmount)
		if i == 0 {
			newAmount = strconv.Itoa(eachAmount + x)
		}
		_, err := c.placeStopOrder(oppositeSide, market, newAmount, target)
		if err != nil {
			return fmt.Errorf("failed to place take profit order at target %s: %v", target, err)
		}
	}

	return nil
}

type placeOrderResponse struct {
	OrderId          string `json:"order_id"`
	Market           string `json:"market"`
	MarketType       string `json:"market_type"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	Amount           string `json:"amount"`
	Price            string `json:"price"`
	UnfilledAmount   string `json:"unfilled_amount"`
	FilledAmount     string `json:"filled_amount"`
	FilledValue      string `json:"filled_value"`
	ClientId         string `json:"client_id"`
	Fee              string `json:"fee"`
	FeeCcy           string `json:"fee_ccy"`
	MakerFeeRate     string `json:"maker_fee_rate"`
	TakerFeeRate     string `json:"taker_fee_rate"`
	LastFilledAmount string `json:"last_filled_amount"`
	LastFilledPrice  string `json:"last_filled_price"`
	RealizedPnl      string `json:"realized_pnl"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}
type CreateOrder struct {
	Market     string `json:"market"`
	MarketType string `json:"market_type"`
	Side       string `json:"side"`
	Type       string `json:"type"`
	Amount     string `json:"amount"`
	Price      string `json:"price"`
}

func (c *engine) placeOrder(side, market, amount, target string) (string, error) {
	//params := fmt.Sprintf("?market=%s&market_type=%s&side=%s&type=%s&amount=%s&price=%s",
	//market, "FUTURES", side, "limit", amount, target)

	req := CreateOrder{
		Market:     market,
		MarketType: "FUTURES",
		Side:       side,
		Type:       "market",
		Amount:     amount,
		Price:      target,
	}
	rr, _ := json.Marshal(req)
	response, err := c.call("/v2/futures/order", "POST", "", rr)
	if err != nil {
		return "", err
	}

	resp2, ok := response.(map[string]interface{})
	if !ok {
		return "", errors.New("response is not a map")
	}

	var resp placeOrderResponse
	err = MapJsonToStruct(resp2, &resp)
	if err != nil {
		return "", err
	}

	return resp.OrderId, nil
}

type placeStopOrderResponse struct {
	StopId int64 `json:"stop_id"`
}

type CreateStopOrder struct {
	Market           string `json:"market"`
	MarketType       string `json:"market_type"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	Amount           string `json:"amount"`
	TriggerPriceType string `json:"trigger_price_type"`
	TriggerPrice     string `json:"trigger_price"`
}

func (c *engine) placeStopOrder(side, market, amount, target string) (string, error) {
	req := CreateStopOrder{
		Market:           market,
		MarketType:       "FUTURES",
		Side:             side,
		Type:             "market",
		Amount:           amount,
		TriggerPriceType: "mark_price",
		TriggerPrice:     target,
	}
	rr, _ := json.Marshal(req)
	response, err := c.call("/v2/futures/stop-order", "POST", "", rr)
	if err != nil {
		return "", err
	}

	resp2, ok := response.(map[string]interface{})
	if !ok {
		return "", errors.New("response is not a map")
	}

	var resp placeStopOrderResponse
	err = MapJsonToStruct(resp2, &resp)
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(resp.StopId, 10), nil
}

type placeTakeProfitRequest struct {
	Market          string `json:"market"`
	MarketType      string `json:"market_type"`
	TakeProfitType  string `json:"take_profit_type"`
	TakeProfitPrice string `json:"take_profit_price"`
}

func (c *engine) placeTP(market, target string) error {
	req := placeTakeProfitRequest{
		Market:          market,
		MarketType:      "FUTURES",
		TakeProfitType:  "mark_price",
		TakeProfitPrice: target,
	}
	rr, _ := json.Marshal(req)
	response, err := c.call("/v2/futures/set-position-take-profit", "POST", "", rr)
	if err != nil {
		return err
	}

	_, ok := response.(map[string]interface{})
	if !ok {
		return errors.New("response is not a map")
	}

	return nil
}

type placeStopLossRequest struct {
	Market        string `json:"market"`
	MarketType    string `json:"market_type"`
	StopLossType  string `json:"stop_loss_type"`
	StopLossPrice string `json:"stop_loss_price"`
}

func (c *engine) placeSL(market, target string) error {
	req := placeStopLossRequest{
		Market:        market,
		MarketType:    "FUTURES",
		StopLossType:  "mark_price",
		StopLossPrice: target,
	}
	rr, _ := json.Marshal(req)
	response, err := c.call("/v2/futures/set-position-stop-loss", "POST", "", rr)
	if err != nil {
		return err
	}

	_, ok := response.(map[string]interface{})
	if !ok {
		return errors.New("response is not a map")
	}

	return nil
}

type Position struct {
	PositionId             int    `json:"position_id"`
	Market                 string `json:"market"`
	MarketType             string `json:"market_type"`
	Side                   string `json:"side"`
	MarginMode             string `json:"margin_mode"`
	OpenInterest           string `json:"open_interest"`
	CloseAvbl              string `json:"close_avbl"`
	AthPositionAmount      string `json:"ath_position_amount"`
	UnrealizedPnl          string `json:"unrealized_pnl"`
	RealizedPnl            string `json:"realized_pnl"`
	AvgEntryPrice          string `json:"avg_entry_price"`
	CmlPositionValue       string `json:"cml_position_value"`
	MaxPositionValue       string `json:"max_position_value"`
	TakeProfitPrice        string `json:"take_profit_price"`
	StopLossPrice          string `json:"stop_loss_price"`
	TakeProfitType         string `json:"take_profit_type"`
	StopLossType           string `json:"stop_loss_type"`
	Leverage               string `json:"leverage"`
	MarginAvbl             string `json:"margin_avbl"`
	AthMarginSize          string `json:"ath_margin_size"`
	PositionMarginRate     string `json:"position_margin_rate"`
	MaintenanceMarginRate  string `json:"maintenance_margin_rate"`
	MaintenanceMarginValue string `json:"maintenance_margin_value"`
	LiqPrice               string `json:"liq_price"`
	BkrPrice               string `json:"bkr_price"`
	AdlLevel               int    `json:"adl_level"`
	SettlePrice            string `json:"settle_price"`
	SettleValue            string `json:"settle_value"`
	CreatedAt              int64  `json:"created_at"`
	UpdatedAt              int64  `json:"updated_at"`
}

type openPositionResponse []Position

func (c *engine) getOpenPosition(market string) ([]Position, error) {
	response, err := c.call("/v2/futures/pending-position", "GET", fmt.Sprintf(
		"?market=%s&market_type=%s",
		market, "FUTURES"), nil)
	if err != nil {
		return nil, err
	}

	resp2, ok := response.([]interface{})
	if !ok {
		return nil, errors.New("response is not a map")
	}

	var resp openPositionResponse
	for _, v := range resp2 {
		var pos Position
		err = MapJsonToStruct(v.(map[string]interface{}), &pos)
		if err != nil {
			return nil, err
		}
		resp = append(resp, pos)
	}

	return resp, nil
}

func (c *engine) calPositionAmount(market string, leverage int) (string, error) {
	info, err := c.marketInfo(market)
	if err != nil {
		return "", err
	}
	lastPrice, err := strconv.ParseFloat(info[0].Last, 64)
	if err != nil {
		return "", err
	}

	balances, err := c.balances()
	if err != nil {
		return "", err
	}

	availableBalance := 0.
	for _, b := range balances {
		if b.Ccy == "USDT" {
			availableBalance, err = strconv.ParseFloat(b.Available, 64)
			if err != nil {
				return "", err
			}
		}
	}
	if availableBalance < 2 {
		return "", errors.New("not enough balance")
	}

	availableBalance = (10 / 100) * availableBalance
	if availableBalance < 2 {
		availableBalance = 2
	}

	// Calculate the position amount based on the available balance
	// and the leverage
	positionAmount := (availableBalance * float64(leverage)) / lastPrice
	return strconv.FormatFloat(positionAmount, 'f', -1, 64), nil
}

type marketInfo struct {
	Market     string `json:"market"`
	Last       string `json:"last"`
	Open       string `json:"open"`
	Close      string `json:"close"`
	High       string `json:"high"`
	Low        string `json:"low"`
	Volume     string `json:"volume"`
	VolumeSell string `json:"volume_sell"`
	VolumeBuy  string `json:"volume_buy"`
	Value      string `json:"value"`
	IndexPrice string `json:"index_price"`
	MarkPrice  string `json:"mark_price"`
	Period     int    `json:"period"`
}

func (c *engine) marketInfo(market string) ([]marketInfo, error) {
	response, err := c.call("/v2/futures/ticker", "GET", fmt.Sprintf(
		"?market=%s",
		market), nil)
	if err != nil {
		return nil, err
	}

	resp2, ok := response.([]interface{})
	if !ok {
		return nil, errors.New("response is not a map")
	}

	var resp []marketInfo
	for _, v := range resp2 {
		var pos marketInfo
		err = MapJsonToStruct(v.(map[string]interface{}), &pos)
		if err != nil {
			return nil, err
		}
		resp = append(resp, pos)
	}

	return resp, nil
}

type balance struct {
	Ccy           string `json:"ccy"`
	Available     string `json:"available"`
	Frozen        string `json:"frozen"`
	Margin        string `json:"margin"`
	UnrealizedPnl string `json:"unrealized_pnl"`
	Transferrable string `json:"transferrable"`
}

func (c *engine) balances() ([]balance, error) {
	response, err := c.call("/v2/assets/futures/balance", "GET", "", nil)
	if err != nil {
		return nil, err
	}

	resp2, ok := response.([]interface{})
	if !ok {
		return nil, errors.New("response is not a map")
	}

	var resp []balance
	for _, v := range resp2 {
		var pos balance
		err = MapJsonToStruct(v.(map[string]interface{}), &pos)
		if err != nil {
			return nil, err
		}
		resp = append(resp, pos)
	}

	return resp, nil
}

type CancelAllOrders struct {
	Market     string `json:"market"`
	MarketType string `json:"market_type"`
}

func (c *engine) cancelAllOrders(market string) error {
	req := CancelAllOrders{
		Market:     market,
		MarketType: "FUTURES",
	}
	rr, _ := json.Marshal(req)
	_, err := c.call("/v2/futures/cancel-all-order", "POST", "", rr)
	if err != nil {
		return err
	}

	return nil
}

func (c *engine) waitPositionOpened(market string) error {
	// Step 3: Monitor the position (simplified, would normally involve more robust logic)

	for {
		time.Sleep(1 / 2 * time.Second) // Simplified monitoring interval

		// Check if the position is still open
		positions, err := c.getOpenPosition(market)
		if err != nil {
			return fmt.Errorf("failed to check position status: %v", err)
		}

		if len(positions) > 0 {
			return nil
		}
	}
}

func (c *engine) waitPositionClosed(market string) error {
	// Step 3: Monitor the position (simplified, would normally involve more robust logic)

	for {
		time.Sleep(1 / 2 * time.Second) // Simplified monitoring interval

		// Check if the position is still open
		positions, err := c.getOpenPosition(market)
		if err != nil {
			return fmt.Errorf("failed to check position status: %v", err)
		}

		if len(positions) == 0 {
			// If position is closed, cancel all open orders in this market
			err := c.cancelAllOrders(market)
			if err != nil {
				return fmt.Errorf("failed to cancel all orders: %v", err)
			}
			break
		}
	}

	return nil
}

func (c *engine) call(url, method, queryParams string, requestBody []byte) (interface{}, error) {
	// Step 1: Generate the timestamp and signature
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	preparedStr := method + url + queryParams + string(requestBody) + timestamp
	signature := generateSignature(c.SecretKey, preparedStr)

	uri := fmt.Sprintf("%s%s%s", coinexBaseURL, url, queryParams)
	// Step 2: Create the HTTP request
	req, err := http.NewRequest(method, uri, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set headers
	req.Header.Set("X-COINEX-KEY", c.ApiKey)
	req.Header.Set("X-COINEX-TIMESTAMP", timestamp)
	req.Header.Set("X-COINEX-SIGN", signature)
	req.Header.Set("Content-Type", "application/json")

	// Step 3: Send the request and handle the response
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to get pending orders
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response["message"] != "OK" {
		return nil, errors.New("got error on calling: " + response["message"].(string))
	}

	// TODO: its not a map some times and instead we are using the array of interface ->>>> []interface{}
	return response["data"], nil
}

func generateSignature(secret, preparedStr string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(preparedStr))
	return hex.EncodeToString(mac.Sum(nil))
}

func MapJsonToStruct(myMap map[string]interface{}, s interface{}) error {
	// Marshal the map to JSON
	jsonData, err := json.Marshal(myMap)
	if err != nil {
		return err
	}

	// Unmarshal JSON data into the struct
	if err := json.Unmarshal(jsonData, s); err != nil {
		return err
	}

	return nil
}
