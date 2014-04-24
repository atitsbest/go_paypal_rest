// Package main provides ...
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-martini/martini"
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
		Id               string    // "PAY-6RV70583SB702805EKEYSZ6Y",
		CreateTime       time.Time `json:"create_time`  // 2013-03-01T22:34:35Z",
		UpdateTime       time.Time `json:"update_time"` // 2013-03-01T22:34:36Z",
		State            string    // created
		Intent           string    // sale
		Payer            PaymentPayer
		Transactions     []PaymentTransaction
		Links            []PaymentLink
		RelatedResources []Resource `json:"related_resources"`
	}

	Resource struct {
		Sale LookupSaleResponse
	}

	LookupSaleResponse struct {
		Id            string    // "36C38912MN9658832",
		CreateTime    time.Time `json:"create_time`  // 2013-03-01T22:34:35Z",
		UpdateTime    time.Time `json:"update_time"` // 2013-03-01T22:34:36Z",
		State         string    // created
		Amount        PaymentAmount
		ParentPayment string `json:"parent_payment"`
		Links         []PaymentLink
	}
)

func (self *PaymentResponse) ApprovalUrl() (string, error) {
	for _, l := range self.Links {
		if l.Rel == "approval_url" {
			return l.Href, nil
		}
	}
	return "", errors.New("No approval_url foundi!")
}

func GetToken(clientId string, secret string) (string, error) {
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

// Macht aus einer authorisierten Bezahlung einen Verkauf.
func ExecuteApprovedPayment(token string, payerId string, paymentId string) (*PaymentResponse, error) {
	url := fmt.Sprintf("https://api.sandbox.paypal.com/v1/payments/payment/%s/execute", paymentId)
	body := strings.NewReader("{ \"payer_id\": \"" + payerId + "\" }")
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := new(http.Client)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		ppres := &PaymentResponse{}
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(ppres)
		if err != nil {
			return nil, err
		}
		return ppres, nil
	}

	return nil, errors.New(res.Status)
}

// Liefert die Details zu einer abgeschlossenen Bezahlung.
func LookupSale(token string, transactionId string) (*LookupSaleResponse, error) {
	url := fmt.Sprintf("https://api.sandbox.paypal.com/v1/payments/sale/%s", transactionId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	client := new(http.Client)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 200 {
		ppres := &LookupSaleResponse{}
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(ppres)
		if err != nil {
			return nil, err
		}
		return ppres, nil
	}

	return nil, errors.New(res.Status)
}

// Wandelt einen float64 in einen String um, wie Paypal ihn braucht.
func toPayPalPrice(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// --- HELPERS ---
func fetchEnvVars() (clientId, secret string) {
	clientId = os.Getenv("PAYPAL_TEST_CLIENTID")

	if len(clientId) <= 0 {
		fmt.Errorf("Für den Test muss die ENV-Variable PAYPAL_TEST_CLIENTID gesetzt sein!")
	}
	secret = os.Getenv("PAYPAL_TEST_SECRET")
	if len(secret) <= 0 {
		fmt.Errorf("Für den Test muss die ENV-Variable PAYPAL_TEST_SECRET gesetzt sein!")
	}
	return
}

func main() {
	var (
		clientId string
		secret   string
		token    string
		err      error
		payment  *PaymentResponse
	)

	m := martini.Classic()
	m.Get("/", func(res http.ResponseWriter, req *http.Request) {
		clientId, secret = fetchEnvVars()
		token, err = GetToken(clientId, secret)
		if err != nil {
			panic(err)
		}

		payment, err = CreatePayPalPayment(
			token,
			1.00, 0.20, 2.00, "USD",
			"Die Dinge die ich eingekauft habe.",
			"http://109.74.200.123:3000/ok", "http://109.74.200.123:3000/cancel")
		if err != nil {
			panic(err)
		}
		// Weiter zu PayPal.
		url, err := payment.ApprovalUrl()
		if err != nil {
			panic(err)
		}
		http.Redirect(res, req, url, http.StatusFound)
	})
	m.Get("/cancel", func() string { return "Bezahlung abgebrochen" })
	m.Get("/ok", func(res http.ResponseWriter, req *http.Request) string {
		params := req.URL.Query()
		// token := params["token"][0]
		payerId := params["PayerID"][0]
		// Bezahlung durchführen.
		sale, err := ExecuteApprovedPayment(token, payerId, payment.Id)
		if err != nil {
			panic(err)
		}

		// if sale.State != "approved" {
		// 	panic(errors.New(fmt.Sprintf("Zahlung ist nicht genehmigt!(%s)", sale.State)))
		// }

		// Bezahlung überprüfen.
		sr, err := LookupSale(token, sale.RelatedResources[0].Sale.Id)
		if err != nil {
			panic(err)
		}

		if sr.State != "completed" {
			panic(errors.New(fmt.Sprintf("Zahlung ist nicht genehmigt! (%s)", sr.State)))
		}

		// TODO: Überprüfen, ob das Geld auch wirklich überwiesen wurde.
		return "Bezahlt!"
	})

	m.Run()
}
