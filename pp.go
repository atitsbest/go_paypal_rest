// Package main provides ...
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type (
	TokenResponse struct {
		Scope        string // "https://api.paypal.com/v1/payments/.* https://api.paypal.com/v1/vault/credit-card https://api.paypal.com/v1/vault/credit-card/.*",
		Access_token string // "EEwJ6tF9x5WCIZDYzyZGaz6Khbw7raYRIBV_WxVvgmsG",
		Token_type   string // "Bearer",
		App_id       string // "APP-6XR95014BA15863X",
		Expires_in   int    // 28800
	}

	// --- PAYMENT REQUEST ---

	PaymentUrls struct {
		ReturnUrl string `json:"return_url"`
		CancelUrl string `json:"cancel_url"`
	}

	PaymentPayer struct {
		PaymentMethod string `json:"payment_method"` // paypal
	}

	PaymentAmount struct {
		Total    string               `json:"total"`    // "3.20"
		Currency string               `json:"currency"` // EUR/USD/...
		Details  PaymentAmountDetails `json:"details"`
	}

	PaymentAmountDetails struct {
		Subtotal string `json:"subtotal"` // "1.00"
		Tax      string `json:"tax"`      // "0.20"
		Shipping string `json:"shipping"` // "2.00"
	}

	PaymentTransaction struct {
		Amount      PaymentAmount `json:"amount"`
		Description string        `json:"description"`
	}

	PaymentRequest struct {
		Intent       string               `json:"intent"` // sale
		RedirectUrls PaymentUrls          `json:"redirect_urls"`
		Payer        PaymentPayer         `json:"payer"`
		Transactions []PaymentTransaction `json:"transactions"`
	}

	// --- PAYMENT RESPONSE ---

	PaymentLink struct {
		Href   string `json:"href"`   // "https://api.sandbox.paypal.com/v1/payments/payment/PAY-6RV70583SB702805EKEYSZ6Y",
		Rel    string `json:"rel"`    // "self",
		Method string `json:"method"` // "GET"
	}

	PaymentResponse struct {
		Id           string    // "PAY-6RV70583SB702805EKEYSZ6Y",
		CreateTime   time.Time `json:"create_time`  // 2013-03-01T22:34:35Z",
		UpdateTime   time.Time `json:"update_time"` // 2013-03-01T22:34:36Z",
		State        string    // created
		Intent       string    // sale
		Payer        PaymentPayer
		Transactions []PaymentTransaction
		Links        []PaymentLink
	}
)

func GetToken(clientId string, secret string) (string, error) {
	fmt.Print("Request erstellen...")
	req, err := http.NewRequest(
		"POST",
		"https://api.sandbox.paypal.com/v1/oauth2/token",
		strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept-Language", "en_US")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientId, secret)

	fmt.Print("Request absetzen...")
	client := new(http.Client)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode == 200 {
		ppres := TokenResponse{}
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(&ppres)
		if err != nil {
			return "", err
		}
		fmt.Printf("#+v", ppres)
		return ppres.Access_token, nil
	}

	return "", errors.New(res.Status)
}

func CreatePayPalPayment(
	token string,
	subtotal float64, tax float64, shipping float64, currency string,
	description string,
	returnUrl string, cancelUrl string) (*PaymentResponse, error) {

	total := subtotal + tax + shipping

	// Request zusammenbauen.

	preq := PaymentRequest{
		Intent: "sale",
		RedirectUrls: PaymentUrls{
			ReturnUrl: returnUrl,
			CancelUrl: cancelUrl,
		},
		Payer: PaymentPayer{
			PaymentMethod: "paypal",
		},
		Transactions: []PaymentTransaction{
			PaymentTransaction{
				Amount: PaymentAmount{
					Total:    toPayPalPrice(total),
					Currency: currency,
					Details: PaymentAmountDetails{
						Subtotal: toPayPalPrice(subtotal),
						Tax:      toPayPalPrice(tax),
						Shipping: toPayPalPrice(shipping),
					},
				},
				Description: description,
			},
		},
	}

	// Http-Request zusammenbauen.
	fmt.Print("Request erstellen...")
	data, err := json.Marshal(preq)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s", string(data))

	req, err := http.NewRequest(
		"POST",
		"https://api.sandbox.paypal.com/v1/payments/payment",
		bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Authorization", "Bearer "+token)

	fmt.Print("Request absetzen...")
	client := new(http.Client)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Http.Response einlesen.
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Http-Response parsen.
	if res.StatusCode == 201 {
		ppres := &PaymentResponse{}
		err = json.Unmarshal(body, &ppres)
		if err != nil {
			return nil, err
		}
		return ppres, nil
	}

	return nil, errors.New(fmt.Sprintf("%s: %s", res.Status, body))
}

// Wandelt einen float64 in einen String um, wie Paypal ihn braucht.
func toPayPalPrice(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}
