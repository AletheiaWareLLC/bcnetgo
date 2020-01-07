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
	"bufio"
	"crypto/rsa"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/financego"
	"github.com/AletheiaWareLLC/netgo"
	"github.com/golang/protobuf/proto"
	"github.com/stripe/stripe-go"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
)

func StripeWebhookHandler(callback func(stripe.Event)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
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
		log.Println("Event", event)

		w.WriteHeader(http.StatusOK)

		callback(event)
	}
}

func RegistrationHandler(aliases *aliasgo.AliasChannel, registrations *bcgo.PoWChannel, node *bcgo.Node, listener bcgo.MiningListener, template *template.Template, publishableKey string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
		switch r.Method {
		case "GET":
			alias := netgo.GetQueryParameter(r.URL.Query(), "alias")
			publicKey := netgo.GetQueryParameter(r.URL.Query(), "publicKey")
			log.Println("Alias", alias)
			log.Println("PublicKey", publicKey)

			data := struct {
				Description string
				Key         string
				Name        string
				Alias       string
			}{
				Description: node.Alias,
				Key:         publishableKey,
				Name:        "Aletheia Ware LLC",
				Alias:       alias,
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

			if len(alias) > 0 && len(stripeEmail) > 0 && len(stripeToken) > 0 {
				if err := bcgo.Pull(aliases, node.Cache, node.Network); err != nil {
					log.Println(err)
				}

				// Get rsa.PublicKey for Alias
				publicKey, err := aliases.GetPublicKey(node.Cache, node.Network, alias[0])
				if err != nil {
					log.Println(err)
					return
				}

				// Create list of access (user + server)
				acl := map[string]*rsa.PublicKey{
					alias[0]:   publicKey,
					node.Alias: &node.Key.PublicKey,
				}
				log.Println("Access", acl)

				stripeCustomer, bcRegistration, err := financego.NewRegistration(node.Alias, alias[0], stripeEmail[0], stripeToken[0], alias[0]+" "+node.Alias)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("StripeCustomer", stripeCustomer)
				log.Println("BcRegistration", bcRegistration)
				registrationData, err := proto.Marshal(bcRegistration)
				if err != nil {
					log.Println(err)
					return
				}

				if err := bcgo.LoadCachedHead(registrations, node.Cache); err != nil {
					log.Println(err)
				}
				if err := bcgo.Pull(registrations, node.Cache, node.Network); err != nil {
					log.Println(err)
				}
				_, err = node.Write(bcgo.Timestamp(), registrations, acl, nil, registrationData)
				if err != nil {
					log.Println(err)
					return
				}

				registrationHash, registrationBlock, err := node.Mine(registrations, listener)
				if err != nil {
					log.Println(err)
					return
				}
				if err := bcgo.Push(registrations, node.Cache, node.Network); err != nil {
					log.Println(err)
				}
				registrationReference := &bcgo.Reference{
					Timestamp:   registrationBlock.Timestamp,
					ChannelName: registrationBlock.ChannelName,
					BlockHash:   registrationHash,
				}
				log.Println("RegistrationReference", registrationReference)

				if len(api) > 0 {
					switch api[0] {
					case "1":
						w.Write([]byte(stripeCustomer.ID))
						w.Write([]byte("\n"))
						return
					case "2":
						if err := bcgo.WriteDelimitedProtobuf(bufio.NewWriter(w), registrationReference); err != nil {
							log.Println(err)
						}
						return
					}
				}
				http.Redirect(w, r, "/registered.html", http.StatusFound)
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}

func SubscriptionHandler(aliases *aliasgo.AliasChannel, subscriptions *bcgo.PoWChannel, node *bcgo.Node, listener bcgo.MiningListener, template *template.Template, redirect, productId, planId string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
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

			if len(alias) > 0 && len(customerId) > 0 {
				if err := bcgo.Pull(aliases, node.Cache, node.Network); err != nil {
					log.Println(err)
				}

				// Get rsa.PublicKey for Alias
				publicKey, err := aliases.GetPublicKey(node.Cache, node.Network, alias[0])
				if err != nil {
					log.Println(err)
					return
				}

				// Create list of access (user + server)
				acl := map[string]*rsa.PublicKey{
					alias[0]:   publicKey,
					node.Alias: &node.Key.PublicKey,
				}
				log.Println("Access", acl)

				stripeSubscription, bcSubscription, err := financego.NewSubscription(node.Alias, alias[0], customerId[0], "", productId, planId)
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("StripeSubscription", stripeSubscription)
				log.Println("BcSubscription", bcSubscription)

				subscriptionData, err := proto.Marshal(bcSubscription)
				if err != nil {
					log.Println(err)
					return
				}

				if err := bcgo.LoadCachedHead(subscriptions, node.Cache); err != nil {
					log.Println(err)
				}
				if err := bcgo.Pull(subscriptions, node.Cache, node.Network); err != nil {
					log.Println(err)
				}
				_, err = node.Write(bcgo.Timestamp(), subscriptions, acl, nil, subscriptionData)
				if err != nil {
					log.Println(err)
					return
				}

				subscriptionHash, subscriptionBlock, err := node.Mine(subscriptions, listener)
				if err != nil {
					log.Println(err)
					return
				}
				if err := bcgo.Push(subscriptions, node.Cache, node.Network); err != nil {
					log.Println(err)
				}
				subscriptionReference := &bcgo.Reference{
					Timestamp:   subscriptionBlock.Timestamp,
					ChannelName: subscriptionBlock.ChannelName,
					BlockHash:   subscriptionHash,
				}
				log.Println("SubscriptionReference", subscriptionReference)

				if len(api) > 0 {
					switch api[0] {
					case "1":
						w.Write([]byte(stripeSubscription.ID))
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
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}
