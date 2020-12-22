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
	"bytes"
	"encoding/base64"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/cryptogo"
	"log"
	"net"
	"sync"
)

func ConnectPortTCPHandler(network *bcgo.TCPNetwork) func(conn net.Conn) {
	return func(conn net.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		data := make([]byte, aliasgo.MAX_ALIAS_LENGTH)
		n, err := reader.Read(data[:])
		if err != nil {
			log.Println(err)
			return
		}
		if n <= 0 {
			log.Println("Could not read data")
			return
		}
		if _, err = writer.Write(data[:n]); err != nil {
			log.Println(err)
			return
		}
		if err = writer.Flush(); err != nil {
			log.Println(err)
			return
		}
		host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(host, port, string(data[:n]))
		if network != nil {
			network.AddPeer(host)
		}
	}
}

func BlockPortTCPHandler(cache bcgo.Cache) func(conn net.Conn) {
	inflight := make(map[string]bool)
	mutex := sync.RWMutex{}
	return func(conn net.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		request := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, request); err != nil {
			log.Println(err)
			return
		}
		blockHash := base64.RawURLEncoding.EncodeToString(request.BlockHash)
		recordHash := base64.RawURLEncoding.EncodeToString(request.RecordHash)
		log.Println("Block Request", conn.RemoteAddr(), request.ChannelName, blockHash, recordHash)
		key := request.ChannelName + blockHash + recordHash
		mutex.Lock()
		if inflight[key] {
			mutex.Unlock()
			log.Println("Block Request Already Inflight")
			return
		}
		inflight[key] = true
		mutex.Unlock()
		defer func() {
			mutex.Lock()
			inflight[key] = false
			mutex.Unlock()
		}()
		hash := request.BlockHash
		if hash != nil && len(hash) > 0 {
			// Read block
			block, err := cache.GetBlock(hash)
			if err != nil {
				log.Println(err)
				return
			}
			// Write to connection
			log.Println("Writing block")
			if err := bcgo.WriteDelimitedProtobuf(writer, block); err != nil {
				log.Println(err)
				return
			}
		} else {
			reference, err := cache.GetHead(request.ChannelName)
			if err != nil {
				log.Println(err)
				return
			}
			hash := request.RecordHash
			if hash != nil && len(hash) > 0 {
				// Search through chain until record hash is found, and return the containing block
				if err := bcgo.Iterate(request.ChannelName, reference.BlockHash, nil, cache, nil, func(h []byte, b *bcgo.Block) error {
					for _, e := range b.Entry {
						if bytes.Equal(e.RecordHash, hash) {
							log.Println("Found record, writing block")
							// Write to connection
							if err := bcgo.WriteDelimitedProtobuf(writer, b); err != nil {
								return err
							}
							return bcgo.StopIterationError{}
						}
					}
					return nil
				}); err != nil {
					switch err.(type) {
					case bcgo.StopIterationError:
						// Do nothing
						break
					default:
						log.Println(err)
						return
					}
				}
			} else {
				log.Println("Missing block hash and record hash")
				return
			}
		}
	}
}

func HeadPortTCPHandler(cache bcgo.Cache) func(conn net.Conn) {
	inflight := make(map[string]bool)
	mutex := sync.RWMutex{}
	return func(conn net.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		request := &bcgo.Reference{}
		if err := bcgo.ReadDelimitedProtobuf(reader, request); err != nil {
			log.Println(err)
			return
		}
		log.Println("Head Request", conn.RemoteAddr(), request.ChannelName)
		key := request.ChannelName
		mutex.Lock()
		if inflight[key] {
			mutex.Unlock()
			log.Println("Head Request Already Inflight")
			return
		}
		inflight[key] = true
		mutex.Unlock()
		defer func() {
			mutex.Lock()
			inflight[key] = false
			mutex.Unlock()
		}()
		reference, err := cache.GetHead(request.ChannelName)
		if err != nil {
			log.Println(err)
			return
		}
		blockHash := base64.RawURLEncoding.EncodeToString(reference.BlockHash)
		log.Println("Head Response", reference.ChannelName, blockHash)
		if err := bcgo.WriteDelimitedProtobuf(writer, reference); err != nil {
			log.Println(err)
			return
		}
	}
}

func BroadcastPortTCPHandler(cache bcgo.Cache, network *bcgo.TCPNetwork, open func(string) (*bcgo.Channel, error)) func(conn net.Conn) {
	inflight := make(map[string]bool)
	mutex := sync.RWMutex{}
	return func(conn net.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		block := &bcgo.Block{}
		if err := bcgo.ReadDelimitedProtobuf(reader, block); err != nil {
			log.Println(err)
			return
		}
		hash, err := cryptogo.HashProtobuf(block)
		if err != nil {
			log.Println(err)
			return
		}
		blockHash := base64.RawURLEncoding.EncodeToString(hash)
		log.Println("Broadcast", conn.RemoteAddr(), block.ChannelName, blockHash)
		key := block.ChannelName + blockHash
		mutex.Lock()
		if inflight[key] {
			mutex.Unlock()
			log.Println("Broadcast Already Inflight")
			return
		}
		inflight[key] = true
		mutex.Unlock()
		defer func() {
			mutex.Lock()
			inflight[key] = false
			mutex.Unlock()
		}()
		channel, err := open(block.ChannelName)
		if err != nil {
			log.Println(err)
			return
		}

		b := block
		for b != nil {
			h := b.Previous
			if h != nil && len(h) > 0 {
				b, err = cache.GetBlock(h)
				if err != nil {
					// Request block from broadcaster
					if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
						ChannelName: channel.Name,
						BlockHash:   h,
					}); err != nil {
						log.Println(err)
						return
					}
					b = &bcgo.Block{}
					if err := bcgo.ReadDelimitedProtobuf(reader, b); err != nil {
						log.Println(err)
						return
					}
					bh, err := cryptogo.HashProtobuf(b)
					if err != nil {
						log.Println(err)
						return
					}
					if !bytes.Equal(h, bh) {
						log.Println("Got wrong block from broadcaster")
						return
					}
					cache.PutBlock(h, b)
				} else {
					break
				}
			} else {
				b = nil
			}
		}

		if err := channel.Update(cache, network, hash, block); err != nil {
			log.Println(err)
			// return - Must send head reference back
		} else {
			host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				log.Println(err)
			} else if network != nil {
				// Add host to network and/or reset error count
				network.AddPeer(host)
			}
		}

		// Reply with current head
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			Timestamp:   channel.Timestamp,
			ChannelName: channel.Name,
			BlockHash:   channel.Head,
		}); err != nil {
			log.Println(err)
			return
		}
	}
}
