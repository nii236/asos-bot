package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	mailgun "gopkg.in/mailgun/mailgun-go.v1"
)

var productID int
var variantID int
var mailTest bool
var mailRecipients string
var mailDomain string
var mailPubkey string
var mailPrivkey string

func init() {
	flag.IntVar(&productID, "product-id", 7785575, "Set the productID")
	flag.IntVar(&variantID, "variant-id", 7785646, "Set the variantID")
	flag.BoolVar(&mailTest, "mail-test", false, "Send an email")
	flag.StringVar(&mailRecipients, "mail-recipients", "", "comma separated recipients")
	flag.StringVar(&mailDomain, "mail-domain", "", "set the mail domain")
	flag.StringVar(&mailPubkey, "mail-pubkey", "", "set the mailgun pubkey")
	flag.StringVar(&mailPrivkey, "mail-privkey", "", "set the mailgun privkey")
	flag.Parse()
}

func main() {
	mg := mailgun.NewMailgun(mailDomain, mailPrivkey, mailPubkey)

	if mailTest {
		recipientSlice := strings.Split(strings.TrimSpace(mailRecipients), ",")

		sender := "noreply@niiteki.io"
		subject := "Niiteki test email"
		body := "This is a test email from the ASOS bot."

		err := sendMessage(mg, sender, subject, body, recipientSlice)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	t := time.NewTicker(1 * time.Hour)
	r := &Result{
		IsInStock:         false,
		PreviousIsInStock: false,
	}

	err := scrapeJSON(r)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Is in stock:", r.IsInStock)

	go func(mg mailgun.Mailgun, r *Result, t *time.Ticker) {
		for {
			select {
			case <-t.C:
				err := scrapeJSON(r)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println("Is in stock:", r.IsInStock)

				if r.IsInStock != r.PreviousIsInStock {
					fmt.Println("CHANGE DETECTED! Notifying...")
					recipientSlice := strings.Split(mailRecipients, ",")

					sender := "noreply@niiteki.io"
					subject := "ASOS coat back in stock!"
					body := `ASOS Item 7785575 Variant 7785646 is back in stock.
Item Link: https://www.asos.com/au/asos/asos-swing-coat-with-full-skirt-and-zip-front/prd/7785575

From the ASOS Bot
`

					if !r.IsInStock {
						subject = "ASOS coat no longer in stock"
						body = `ASOS Item 7785575 Variant 7785646 is no longer in stock.
Item Link: https://www.asos.com/au/asos/asos-swing-coat-with-full-skirt-and-zip-front/prd/7785575

From the ASOS Bot
`
					}

					err = sendMessage(mg, sender, subject, body, recipientSlice)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}

	}(mg, r, t)
	select {}
}
func sendMessage(mg mailgun.Mailgun, sender, subject, body string, recipient []string) error {
	message := mg.NewMessage(sender, subject, body, recipient...)
	resp, id, err := mg.Send(message)

	if err != nil {
		return err
	}

	fmt.Printf("Email sent: ID: %s Resp: %s\n", id, resp)
	return nil
}
func scrapeJSON(r *Result) error {
	client := http.DefaultClient
	client.Timeout = 5 * time.Second
	resp, err := client.Get(fmt.Sprintf("http://m.asos.com/api/product/catalogue/v2/stockprice?productIds=%d&currency=AUD&keyStoreDataversion=5e950e2a-9&store=AU", productID))
	if err != nil {
		return err
	}
	data := API{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return err
	}
	if len(data) < 1 {
		return errors.New("no values in slice")
	}
	v, err := data.Variant(7785646)
	if err != nil {
		return err
	}

	r.PreviousIsInStock = r.IsInStock
	r.IsInStock = v.IsInStock

	return nil
}

// func scrapePage(r *Result) error {
// 	u, err := url.Parse("http://m.asos.com/au/asos/asos-swing-coat-with-full-skirt-and-zip-front/prd/7785575")
// 	if err != nil {
// 		return err
// 	}
// 	doc, err := goquery.NewDocument(u.String())
// 	if err != nil {
// 		return err
// 	}
// 	scripts := doc.Find("body > script")
// 	var data *PageJSON
// 	scripts.Each(func(i int, sel *goquery.Selection) {
// 		htmlStr, err := sel.Html()
// 		if err != nil {
// 			fmt.Println(err)
// 			return
// 		}
// 		unescaped := html.UnescapeString(htmlStr)
// 		if strings.Contains(unescaped, "PageConfig") {
// 			fmt.Println("FOUND")
// 			r := regexp.MustCompile(`'({.*})'`)
// 			matches := r.FindAllStringSubmatch(unescaped, 1)

// 			err = json.Unmarshal([]byte(matches[0][1]), &data)
// 			if err != nil {
// 				fmt.Println(err)
// 				return
// 			}
// 		}
// 	})
// 	if data == nil {
// 		return errors.New("pageConfig not found")
// 	}

// 	return nil
// }

type Variant struct {
	VariantID    int    `json:"variantId"`
	Sku          string `json:"sku"`
	IsInStock    bool   `json:"isInStock"`
	IsLowInStock bool   `json:"isLowInStock"`
}

func (api API) Variant(id int) (*Variant, error) {
	for _, prdt := range api {
		for _, v := range prdt.Variants {
			if v.VariantID == id {
				return &Variant{
					VariantID:    v.VariantID,
					Sku:          v.Sku,
					IsInStock:    v.IsInStock,
					IsLowInStock: v.IsLowInStock,
				}, nil
			}
		}
	}

	return nil, errors.New("could not find variant")
}

type Result struct {
	PreviousIsInStock bool
	IsInStock         bool
}

// type PageJSON struct {
// 	ID          int    `json:"id"`
// 	Name        string `json:"name"`
// 	BrandName   string `json:"brandName"`
// 	SizeGuide   string `json:"sizeGuide"`
// 	ProductCode string `json:"productCode"`
// 	Price       struct {
// 		Current  float64 `json:"current"`
// 		Previous float64 `json:"previous"`
// 		Rrp      float64 `json:"rrp"`
// 		Currency string  `json:"currency"`
// 	} `json:"price"`
// 	Media struct {
// 		CatwalkURL    string `json:"catwalkUrl"`
// 		ThreeSixtyURL string `json:"threeSixtyUrl"`
// 	} `json:"media"`
// 	Images []struct {
// 		ProductID     int    `json:"productId"`
// 		URL           string `json:"url"`
// 		Colour        string `json:"colour"`
// 		ColourCode    string `json:"colourCode"`
// 		IsPrimary     bool   `json:"isPrimary"`
// 		AlternateText string `json:"alternateText"`
// 		IsVisible     bool   `json:"isVisible"`
// 		ImageType     string `json:"imageType"`
// 	} `json:"images"`
// 	ColourImageMap struct {
// 		Mink int `json:"mink"`
// 	} `json:"colourImageMap"`
// 	LocalisedColourImageMap struct {
// 		Mink int `json:"mink"`
// 	} `json:"localisedColourImageMap"`
// 	Variants []struct {
// 		VariantID  int    `json:"variantId"`
// 		Size       string `json:"size"`
// 		SizeID     int    `json:"sizeId"`
// 		Colour     string `json:"colour"`
// 		ColourCode string `json:"colourCode"`
// 		IsPrimary  bool   `json:"isPrimary"`
// 		SizeOrder  int    `json:"sizeOrder"`
// 	} `json:"variants"`
// 	Categories           []interface{} `json:"categories"`
// 	CompleteTheLookURL   interface{}   `json:"completeTheLookUrl"`
// 	MixAndMatchURL       interface{}   `json:"mixAndMatchUrl"`
// 	SizeGuideVisible     bool          `json:"sizeGuideVisible"`
// 	IsInStock            bool          `json:"isInStock"`
// 	IsNoSize             bool          `json:"isNoSize"`
// 	IsOneSize            bool          `json:"isOneSize"`
// 	Gender               string        `json:"gender"`
// 	ShippingRestrictions struct {
// 		ShippingRestrictionsURL     interface{} `json:"shippingRestrictionsUrl"`
// 		ShippingRestrictionsLabel   interface{} `json:"shippingRestrictionsLabel"`
// 		ShippingRestrictionsVisible bool        `json:"shippingRestrictionsVisible"`
// 	} `json:"shippingRestrictions"`
// 	Store struct {
// 		ID                        int    `json:"id"`
// 		Code                      string `json:"code"`
// 		URL                       string `json:"url"`
// 		Language                  string `json:"language"`
// 		LanguageShort             string `json:"languageShort"`
// 		SizeSchema                string `json:"sizeSchema"`
// 		Currency                  string `json:"currency"`
// 		CountryCode               string `json:"countryCode"`
// 		KeyStoreDataversion       string `json:"keyStoreDataversion"`
// 		SiteChromeTemplateVersion string `json:"siteChromeTemplateVersion"`
// 	} `json:"store"`
// 	BuyTheLookID int `json:"buyTheLookId"`
// }
type API []struct {
	ProductID    int    `json:"productId"`
	ProductCode  string `json:"productCode"`
	ProductPrice struct {
		Current struct {
			Value float64 `json:"value"`
			Text  string  `json:"text"`
		} `json:"current"`
		Previous struct {
			Value float64 `json:"value"`
			Text  string  `json:"text"`
		} `json:"previous"`
		Rrp struct {
			Value float64 `json:"value"`
			Text  string  `json:"text"`
		} `json:"rrp"`
		Xrp struct {
			Value float64 `json:"value"`
			Text  string  `json:"text"`
		} `json:"xrp"`
		Currency      string `json:"currency"`
		IsMarkedDown  bool   `json:"isMarkedDown"`
		IsOutletPrice bool   `json:"isOutletPrice"`
	} `json:"productPrice"`
	Variants []struct {
		VariantID    int    `json:"variantId"`
		Sku          string `json:"sku"`
		IsInStock    bool   `json:"isInStock"`
		IsLowInStock bool   `json:"isLowInStock"`
		Price        struct {
			Current struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"current"`
			Previous struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"previous"`
			Rrp struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"rrp"`
			Xrp struct {
				Value float64 `json:"value"`
				Text  string  `json:"text"`
			} `json:"xrp"`
			Currency      string `json:"currency"`
			IsMarkedDown  bool   `json:"isMarkedDown"`
			IsOutletPrice bool   `json:"isOutletPrice"`
		} `json:"price"`
	} `json:"variants"`
}
