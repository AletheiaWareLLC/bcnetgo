/*
 * Copyright 2019 Aletheia Ware LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bcnetgo

import (
	"aletheiaware.com/bcgo"
	"aletheiaware.com/financego"
	"aletheiaware.com/netgo"
	"bufio"
	"fmt"
	"github.com/stripe/stripe-go"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

func StripeWebhookHandler(callback func(*stripe.Event)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path, r.Header)
		log.Println("Stripe Webhook", r)
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			return
		}
		event, err := financego.ConstructEvent(data, r.Header.Get("Stripe-Signature"))
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Stripe Event", event)
		/*
			log.Println("Account", event.Account)
			log.Println("Created", event.Created)
			log.Println("Data Object", event.Data.Object)
			log.Println("Data PreviousAttributes", event.Data.PreviousAttributes)
			log.Println("ID", event.ID)
			log.Println("Livemode", event.Livemode)
			log.Println("PendingWebhooks", event.PendingWebhooks)
			log.Println("Request ID", event.Request.ID)
			log.Println("Request IdempotencyKey", event.Request.IdempotencyKey)
			log.Println("Type", event.Type)
		*/

		w.WriteHeader(http.StatusOK)

		callback(&event)
	}
}

func RegistrationHandler(merchantAlias, merchantName, merchantKey string, template *template.Template, callback func(string, string, string) (string, *bcgo.Reference, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path, r.Header)
		switch r.Method {
		case "GET":
			alias := netgo.GetQueryParameter(r.URL.Query(), "alias")
			next := netgo.GetQueryParameter(r.URL.Query(), "next")
			log.Println("Alias", alias)
			log.Println("Next", next)

			data := struct {
				Description string
				Key         string
				Name        string
				Alias       string
				Next        string
			}{
				Description: merchantAlias,
				Key:         merchantKey,
				Name:        merchantName,
				Alias:       alias,
				Next:        next,
			}
			if err := template.Execute(w, data); err != nil {
				log.Println(err)
				return
			}
		case "POST":
			r.ParseForm()
			api := r.Form["api"]
			alias := r.Form["alias"]
			stripeEmail := r.Form["stripeEmail"]
			// stripeBillingName := r.Form["stripeBillingName"]
			// stripeBillingAddressLine1 := r.Form["stripeBillingAddressLine1"]
			// stripeBillingAddressCity := r.Form["stripeBillingAddressCity"]
			// stripeBillingAddressZip := r.Form["stripeBillingAddressZip"]
			// stripeBillingAddressCountry := r.Form["stripeBillingAddressCountry"]
			// stripeBillingAddressCountryCode := r.Form["stripeBillingAddressCountryCode"]
			// stripeBillingAddressState := r.Form["stripeBillingAddressState"]
			stripeToken := r.Form["stripeToken"]
			// stripeTokenType := r.Form["stripeTokenType"]
			next := r.Form["next"]

			if len(alias) == 0 {
				log.Println("Missing Alias")
				return
			}
			if len(stripeEmail) == 0 {
				log.Println("Missing Stripe Email")
				return
			}
			if len(stripeToken) == 0 {
				log.Println("Missing Stripe Token")
				return
			}

			customerID, registrationReference, err := callback(alias[0], stripeEmail[0], stripeToken[0])
			if err != nil {
				log.Println(err)
				return
			}

			if len(api) > 0 {
				switch api[0] {
				case "1":
					w.Write([]byte(customerID))
					w.Write([]byte("\n"))
					return
				case "2":
					if err := bcgo.WriteDelimitedProtobuf(bufio.NewWriter(w), registrationReference); err != nil {
						log.Println(err)
					}
					return
				}
			}
			if len(next) > 0 {
				http.Redirect(w, r, fmt.Sprintf("%s?alias=%s&customerId=%s", next[0], alias[0], customerID), http.StatusFound)
			}
			http.Redirect(w, r, "/registered.html", http.StatusFound)
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}

func SubscriptionHandler(template *template.Template, redirect string, callback func(string, string) (string, *bcgo.Reference, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path, r.Header)
		switch r.Method {
		case "GET":
			alias := netgo.GetQueryParameter(r.URL.Query(), "alias")
			customerId := netgo.GetQueryParameter(r.URL.Query(), "customerId")
			log.Println("Alias", alias)
			log.Println("Customer ID", customerId)
			data := struct {
				Alias      string
				CustomerId string
			}{
				Alias:      alias,
				CustomerId: customerId,
			}
			if err := template.Execute(w, data); err != nil {
				log.Println(err)
				return
			}
		case "POST":
			r.ParseForm()
			api := r.Form["api"]
			alias := r.Form["alias"]
			customerId := r.Form["customerId"]

			if len(alias) == 0 {
				log.Println("Missing Alias")
				return
			}

			if len(customerId) == 0 {
				log.Println("Missing Customer ID")
				return
			}

			subscriptionID, subscriptionReference, err := callback(alias[0], customerId[0])
			if err != nil {
				log.Println(err)
				return
			}

			if len(api) > 0 {
				switch api[0] {
				case "1":
					w.Write([]byte(subscriptionID))
					w.Write([]byte("\n"))
					return
				case "2":
					if err := bcgo.WriteDelimitedProtobuf(bufio.NewWriter(w), subscriptionReference); err != nil {
						log.Println(err)
					}
					return
				}
			}
			http.Redirect(w, r, redirect, http.StatusFound)
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}
