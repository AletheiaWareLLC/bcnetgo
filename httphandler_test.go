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
	// "github.com/AletheiaWareLLC/bcnetgo"
	"net/http"
	// "net/http/httptest"
	"testing"
)

func TestBlockHandler(t *testing.T) {
	t.Run("BlockExists", func(t *testing.T) {
		// TODO
	})
	t.Run("BlockNotExists", func(t *testing.T) {
		// TODO
	})
}

func makeGetBlockRequest(channel, hash string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/block?channel="+channel+"hash="+hash, nil)
	return request
}

func TestChannelHandler(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		// TODO
	})
	t.Run("NotExists", func(t *testing.T) {
		// TODO
	})
}

func makeGetChannelRequest(channel string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/channel?channel="+channel, nil)
	return request
}

func TestChannelListHandler(t *testing.T) {
	t.Run("None", func(t *testing.T) {
		// TODO
	})
	t.Run("One", func(t *testing.T) {
		// TODO
	})
	t.Run("Many", func(t *testing.T) {
		// TODO
	})
}

func makeGetChannelListRequest(channel string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/channels", nil)
	return request
}
