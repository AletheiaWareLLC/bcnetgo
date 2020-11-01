/*
 * Copyright 2020 Aletheia Ware LLC
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

package bcnetgo_test

import (
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/bcnetgo"
	"github.com/AletheiaWareLLC/testinggo"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	REGISTRATION_TEMPLATE = `Description:{{ .Description }} Key:{{ .Key }} Name:{{ .Name }} Alias:{{ .Alias }} Next:{{ .Next }}`
	SUBSCRIPTION_TEMPLATE = `Alias:{{ .Alias }} Customer ID:{{ .CustomerId }}`
)

func TestStripeWebhookHandler(t *testing.T) {
	// TODO handler := StripeWebhookHandler(callback func(*stripe.Event))
}

func TestRegistrationHandler(t *testing.T) {
	templ, err := template.New("RegisterTest").Parse(REGISTRATION_TEMPLATE)
	testinggo.AssertNoError(t, err)
	t.Run("Get", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/register?alias=Tester", nil)
		response := httptest.NewRecorder()
		handler := bcnetgo.RegistrationHandler("merchant123", "Merchant 123", "key123", templ, func(alias string, email string, token string) (string, *bcgo.Reference, error) {
			log.Println("Customer:", alias, email, token)
			return "cus123", &bcgo.Reference{}, nil
		})
		handler(response, request)

		expected := "Description:merchant123 Key:key123 Name:Merchant 123 Alias:Tester Next:"
		got := response.Body.String()

		if got != expected {
			t.Errorf("Incorrect response; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("Post", func(t *testing.T) {
		data := url.Values{}
		data.Set("alias", "Tester")
		data.Set("api", "1")
		data.Set("stripeEmail", "te@s.t")
		data.Set("stripeToken", "1234")
		request, err := http.NewRequest(http.MethodPost, "/register", strings.NewReader(data.Encode()))
		testinggo.AssertNoError(t, err)
		request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler := bcnetgo.RegistrationHandler("merchant123", "Merchant 123", "key123", templ, func(alias string, email string, token string) (string, *bcgo.Reference, error) {
			log.Println("Customer:", alias, email, token)
			return "cus123", &bcgo.Reference{}, nil
		})
		handler(response, request)

		expected := "cus123\n"
		got := response.Body.String()

		if got != expected {
			t.Errorf("Incorrect response; expected '%s', got '%s'", expected, got)
		}
	})
}

func TestSubscriptionHandler(t *testing.T) {
	templ, err := template.New("SubscribeTest").Parse(SUBSCRIPTION_TEMPLATE)
	testinggo.AssertNoError(t, err)
	t.Run("Get", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/subscribe?alias=Tester&customerId=cust1234", nil)
		response := httptest.NewRecorder()
		handler := bcnetgo.SubscriptionHandler(templ, "/subscribed", func(alias string, customerId string) (string, *bcgo.Reference, error) {
			log.Println("Customer:", alias, customerId)
			return "subItem1234", &bcgo.Reference{}, nil
		})
		handler(response, request)

		expected := "Alias:Tester Customer ID:cust1234"
		got := response.Body.String()

		if got != expected {
			t.Errorf("Incorrect response; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("Post", func(t *testing.T) {
		data := url.Values{}
		data.Set("alias", "Tester")
		data.Set("api", "1")
		data.Set("customerId", "cust1234")
		request, err := http.NewRequest(http.MethodPost, "/subscribe", strings.NewReader(data.Encode()))
		testinggo.AssertNoError(t, err)
		request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler := bcnetgo.SubscriptionHandler(templ, "/subscribed", func(alias string, customerId string) (string, *bcgo.Reference, error) {
			log.Println("Customer:", alias, customerId)
			return "subItem1234", &bcgo.Reference{}, nil
		})
		handler(response, request)

		expected := "subItem1234\n"
		got := response.Body.String()

		if got != expected {
			t.Errorf("Incorrect response; expected '%s', got '%s'", expected, got)
		}
	})
}
