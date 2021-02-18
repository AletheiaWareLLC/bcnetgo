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
	"aletheiaware.com/bcgo"
	"aletheiaware.com/bcnetgo"
	"aletheiaware.com/cryptogo"
	"bufio"
	"encoding/base64"
	"errors"
	"net"
	"testing"
	"time"
)

func makeNetwork(t *testing.T) *bcgo.TCPNetwork {
	t.Helper()
	return &bcgo.TCPNetwork{
		DialTimeout: time.Second,
		GetTimeout:  time.Second,
	}
}

func TestConnectPortTCPHandler(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		expected := "Foobar"
		var got string
		network := makeNetwork(t)
		handler := bcnetgo.ConnectPortTCPHandler(network, func(address, peer string) bool {
			got = peer
			return true // Allowed
		})
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		writer := bufio.NewWriter(client)
		writer.Write([]byte(expected))
		writer.Flush()

		time.Sleep(time.Second)

		if got != expected {
			t.Fatalf("Incorrect peer; expected '%s', got '%s'", expected, got)
		}
		found := false
		for _, p := range network.Peers() {
			if p == expected {
				found = true
			}
		}
		if !found {
			t.Fatalf("Expected peer to be added to network")
		}
	})
	t.Run("Disallowed", func(t *testing.T) {
		expected := "Foobar"
		var got string
		network := makeNetwork(t)
		handler := bcnetgo.ConnectPortTCPHandler(network, func(address, peer string) bool {
			got = peer
			return false // Disallowed
		})
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		writer := bufio.NewWriter(client)
		writer.Write([]byte(expected))
		writer.Flush()

		time.Sleep(time.Second)

		if got != expected {
			t.Fatalf("Incorrect peer; expected '%s', got '%s'", expected, got)
		}
		found := false
		for _, p := range network.Peers() {
			if p == expected {
				found = true
			}
		}
		if found {
			t.Fatalf("Expected peer to not be added to network")
		}
	})
}

func TestBlockPortTCPHandler(t *testing.T) {
	t.Run("BlockExists", func(t *testing.T) {
		serverBlock := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		serverHash, err := cryptogo.HashProtobuf(serverBlock)
		if err != nil {
			t.Fatal(err)
		}

		cache := bcgo.NewMemoryCache(10)
		cache.PutBlock(serverHash, serverBlock)
		cache.PutHead("Test", &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   serverHash,
		})
		handler := bcnetgo.BlockPortTCPHandler(cache)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write block request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   serverHash,
		}); err != nil {
			t.Fatal(err)
		}

		// Read block from client conn
		block := &bcgo.Block{}
		if err := bcgo.ReadDelimitedProtobuf(reader, block); err != nil {
			t.Fatal(err)
		}
		hash, err := cryptogo.HashProtobuf(block)
		if err != nil {
			t.Fatal(err)
		}

		expected := base64.RawURLEncoding.EncodeToString(serverHash)
		got := base64.RawURLEncoding.EncodeToString(hash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}

		if serverBlock.String() != block.String() {
			t.Fatalf("Incorrect block; expected '%s', got '%s'", serverBlock.String(), block.String())
		}
	})
	t.Run("BlockNotExists", func(t *testing.T) {
		cache := bcgo.NewMemoryCache(10)
		handler := bcnetgo.BlockPortTCPHandler(cache)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write block request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   []byte("FooBar123"),
		}); err != nil {
			t.Fatal(err)
		}

		// Read block from client conn
		block := &bcgo.Block{}
		if err := bcgo.ReadDelimitedProtobuf(reader, block); err == nil {
			t.Fatal("Expected error")
		}
	})
	t.Run("RecordExists", func(t *testing.T) {
		// TODO
	})
	t.Run("RecordNotExists", func(t *testing.T) {
		// TODO
	})
}

func TestHeadPortTCPHandler(t *testing.T) {
	t.Run("HeadExists", func(t *testing.T) {
		serverBlock := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		serverHash, err := cryptogo.HashProtobuf(serverBlock)
		if err != nil {
			t.Fatal(err)
		}
		serverHead := &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   serverHash,
		}
		cache := bcgo.NewMemoryCache(10)
		cache.PutBlock(serverHash, serverBlock)
		cache.PutHead("Test", serverHead)
		handler := bcnetgo.HeadPortTCPHandler(cache)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write head request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			ChannelName: "Test",
		}); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		expected := base64.RawURLEncoding.EncodeToString(serverHead.BlockHash)
		got := base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("HeadNotExists", func(t *testing.T) {
		cache := bcgo.NewMemoryCache(10)
		handler := bcnetgo.HeadPortTCPHandler(cache)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write head request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			ChannelName: "Test",
		}); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err == nil {
			t.Fatal("Expected error")
		}
	})
}

func TestBroadcastPortTCPHandler(t *testing.T) {
	t.Run("NoSuchChannel", func(t *testing.T) {
		open := func(name string) (*bcgo.Channel, error) {
			return nil, errors.New("No such channel")
		}
		cache := bcgo.NewMemoryCache(10)
		network := makeNetwork(t)
		handler := bcnetgo.BroadcastPortTCPHandler(cache, network, open)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write broadcast request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err == nil {
			t.Fatal("Expected error")
		}
	})
	t.Run("ClientLongerThanServer", func(t *testing.T) {
		clientBlock := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		clientHash, err := cryptogo.HashProtobuf(clientBlock)
		if err != nil {
			t.Fatal(err)
		}
		cache := bcgo.NewMemoryCache(10)
		channel := bcgo.NewChannel("Test")
		open := func(name string) (*bcgo.Channel, error) {
			if name == "Test" {
				return channel, nil
			}
			return nil, errors.New("No such channel")
		}
		network := makeNetwork(t)
		handler := bcnetgo.BroadcastPortTCPHandler(cache, network, open)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write broadcast request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, clientBlock); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		expected := base64.RawURLEncoding.EncodeToString(clientHash)
		got := base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("ClientLongerThanServerMissingBlocks", func(t *testing.T) {
		clientBlock1 := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		clientHash1, err := cryptogo.HashProtobuf(clientBlock1)
		if err != nil {
			t.Fatal(err)
		}
		clientBlock2 := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      2,
			Previous:    clientHash1,
		}
		clientHash2, err := cryptogo.HashProtobuf(clientBlock2)
		if err != nil {
			t.Fatal(err)
		}
		cache := bcgo.NewMemoryCache(10)
		channel := bcgo.NewChannel("Test")
		open := func(name string) (*bcgo.Channel, error) {
			if name == "Test" {
				return channel, nil
			}
			return nil, errors.New("No such channel")
		}
		network := makeNetwork(t)
		handler := bcnetgo.BroadcastPortTCPHandler(cache, network, open)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write broadcast request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, clientBlock2); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		// Expect server to be missing clientBlock1
		expected := base64.RawURLEncoding.EncodeToString(clientHash1)
		got := base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}

		// Write clientBlock1 to server
		if err := bcgo.WriteDelimitedProtobuf(writer, clientBlock1); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head = &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		// Expect server head to be updated to clientHash2
		expected = base64.RawURLEncoding.EncodeToString(clientHash2)
		got = base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("ServerClientEqualLength", func(t *testing.T) {
		serverBlock := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		serverHash, err := cryptogo.HashProtobuf(serverBlock)
		if err != nil {
			t.Fatal(err)
		}
		cache := bcgo.NewMemoryCache(10)
		cache.PutBlock(serverHash, serverBlock)
		cache.PutHead("Test", &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   serverHash,
		})
		channel := bcgo.NewChannel("Test")
		if err := channel.LoadHead(cache, nil); err != nil {
			t.Fatal(err)
		}
		open := func(name string) (*bcgo.Channel, error) {
			if name == "Test" {
				return channel, nil
			}
			return nil, errors.New("No such channel")
		}
		network := makeNetwork(t)
		handler := bcnetgo.BroadcastPortTCPHandler(cache, network, open)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write broadcast request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Block{
			Timestamp:   2345,
			ChannelName: "Test",
			Length:      1,
		}); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		expected := base64.RawURLEncoding.EncodeToString(serverHash)
		got := base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}
	})
	t.Run("ServerLongerThanClient", func(t *testing.T) {
		serverBlock1 := &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}
		serverHash1, err := cryptogo.HashProtobuf(serverBlock1)
		if err != nil {
			t.Fatal(err)
		}
		serverBlock2 := &bcgo.Block{
			Timestamp:   5678,
			ChannelName: "Test",
			Length:      2,
			Previous:    serverHash1,
		}
		serverHash2, err := cryptogo.HashProtobuf(serverBlock2)
		if err != nil {
			t.Fatal(err)
		}
		cache := bcgo.NewMemoryCache(10)
		cache.PutBlock(serverHash1, serverBlock1)
		cache.PutBlock(serverHash2, serverBlock2)
		cache.PutHead("Test", &bcgo.Reference{
			ChannelName: "Test",
			BlockHash:   serverHash2,
		})
		channel := bcgo.NewChannel("Test")
		if err := channel.LoadHead(cache, nil); err != nil {
			t.Fatal(err)
		}
		open := func(name string) (*bcgo.Channel, error) {
			if name == "Test" {
				return channel, nil
			}
			return nil, errors.New("No such channel")
		}
		network := makeNetwork(t)
		handler := bcnetgo.BroadcastPortTCPHandler(cache, network, open)
		server, client := net.Pipe()
		defer client.Close()

		// Start server in goroutine
		go handler(server)

		// Write broadcast request to client conn
		reader := bufio.NewReader(client)
		writer := bufio.NewWriter(client)
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Block{
			Timestamp:   1234,
			ChannelName: "Test",
			Length:      1,
		}); err != nil {
			t.Fatal(err)
		}

		// Read head from client conn
		head := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, head); err != nil {
			t.Fatal(err)
		}

		expected := base64.RawURLEncoding.EncodeToString(serverHash2)
		got := base64.RawURLEncoding.EncodeToString(head.BlockHash)
		if expected != got {
			t.Fatalf("Incorrect hash; expected '%s', got '%s'", expected, got)
		}
	})
}
