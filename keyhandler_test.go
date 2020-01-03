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

package bcnetgo_test

import (
	"encoding/base64"
	"github.com/AletheiaWareLLC/bcnetgo"
	"github.com/AletheiaWareLLC/cryptogo"
	"github.com/golang/protobuf/proto"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestKeyHandler(t *testing.T) {
	t.Run("GETExists", func(t *testing.T) {
		kss := make(bcnetgo.KeyShareStore)
		ks := &cryptogo.KeyShare{
			Name: "Alice",
		}
		kss["Alice"] = ks
		handler := bcnetgo.KeyShareHandler(kss, 0)
		request := makeGetRequest()
		response := httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusOK, response.Code)
		}

		expected := ks.String()
		got := &cryptogo.KeyShare{}
		if err := proto.Unmarshal(response.Body.Bytes(), got); err != nil {
			t.Fatal(err)
		}

		if got.String() != expected {
			t.Fatalf("Incorrect response; expected '%s', got '%s'", expected, got.String())
		}
	})
	t.Run("GETNotExists", func(t *testing.T) {
		kss := make(bcnetgo.KeyShareStore)
		handler := bcnetgo.KeyShareHandler(kss, 0)
		request := makeGetRequest()
		response := httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusNotFound, response.Code)
		}

		expected := ""
		got := response.Body.String()

		if got != expected {
			t.Fatalf("Incorrect response; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("POST", func(t *testing.T) {
		kss := make(bcnetgo.KeyShareStore)
		handler := bcnetgo.KeyShareHandler(kss, 0)
		request := makePostRequest()
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusOK, response.Code)
		}

		expected := ""
		got := response.Body.String()

		if got != expected {
			t.Fatalf("Incorrect response; expected '%s', got '%s'", expected, got)
		}

		ks, ok := kss["Alice"]
		if !ok {
			t.Fatal("KeyShare not stored in KeyShareStore")
		}

		log.Println(ks)
		if ks.Name != "Alice" {
			t.Fatalf("Incorrect KeyShare name; expected 'Alice', got '%s'", ks.Name)
		}
		if string(ks.PublicKey) != "Foo" {
			t.Fatalf("Incorrect KeyShare public key; expected 'Foo', got '%s'", string(ks.PublicKey))
		}
		if ks.PublicFormat.String() != "PKIX" {
			t.Fatalf("Incorrect KeyShare public key format; expected 'PKIX', got '%s'", ks.PublicFormat.String())
		}
		if string(ks.PrivateKey) != "Bar" {
			t.Fatalf("Incorrect KeyShare private key; expected 'Bar', got '%s'", string(ks.PrivateKey))
		}
		if ks.PrivateFormat.String() != "PKCS8" {
			t.Fatalf("Incorrect KeyShare private key format; expected 'PKCS8', got '%s'", ks.PrivateFormat.String())
		}
		if string(ks.Password) != "FooBar" {
			t.Fatalf("Incorrect KeyShare password; expected 'FooBar', got '%s'", string(ks.Password))
		}
	})
	t.Run("Expiry", func(t *testing.T) {
		kss := make(bcnetgo.KeyShareStore)
		handler := bcnetgo.KeyShareHandler(kss, time.Second)
		request := makePostRequest()
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusOK, response.Code)
		}

		// Try get - should succeed
		request = makeGetRequest()
		response = httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusOK, response.Code)
		}

		time.Sleep(3 * time.Second)

		// Try get - should fail
		request = makeGetRequest()
		response = httptest.NewRecorder()

		handler(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("Incorrect response; expected '%d', got '%d'", http.StatusNotFound, response.Code)
		}
	})
}

func makeGetRequest() *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/keys?name=Alice", nil)
	return request
}

func makePostRequest() *http.Request {
	request, _ := http.NewRequest(http.MethodPost, "/keys", strings.NewReader(url.Values{
		"name":             {"Alice"},
		"publicKey":        {base64.RawURLEncoding.EncodeToString([]byte("Foo"))},
		"publicKeyFormat":  {"PKIX"},
		"privateKey":       {base64.RawURLEncoding.EncodeToString([]byte("Bar"))},
		"privateKeyFormat": {"PKCS8"},
		"password":         {base64.RawURLEncoding.EncodeToString([]byte("FooBar"))},
	}.Encode()))
	return request
}
