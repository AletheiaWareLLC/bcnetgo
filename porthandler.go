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
	"github.com/AletheiaWareLLC/bcgo"
	"log"
	"net"
)

func BlockPortHandler(cache bcgo.Cache, network bcgo.Network) func(conn net.Conn) {
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
		hash := request.BlockHash
		if hash != nil && len(hash) > 0 {
			// Read block
			block, err := bcgo.GetBlock(request.ChannelName, cache, network, hash)
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
			reference, err := bcgo.GetHeadReference(request.ChannelName, cache, network)
			if err != nil {
				log.Println(err)
				return
			}
			hash := request.RecordHash
			if hash != nil && len(hash) > 0 {
				// Search through chain until record hash is found, and return the containing block
				if err := bcgo.Iterate(request.ChannelName, reference.BlockHash, nil, cache, network, func(h []byte, b *bcgo.Block) error {
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

func HeadPortHandler(cache bcgo.Cache, network bcgo.Network) func(conn net.Conn) {
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
		reference, err := bcgo.GetHeadReference(request.ChannelName, cache, network)
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

func BroadcastPortHandler(cache bcgo.Cache, network bcgo.Network, open func(string) (bcgo.Channel, error)) func(conn net.Conn) {
	return func(conn net.Conn) {
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		block := &bcgo.Block{}
		if err := bcgo.ReadDelimitedProtobuf(reader, block); err != nil {
			log.Println(err)
			return
		}
		hash, err := bcgo.HashProtobuf(block)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Broadcast", conn.RemoteAddr(), block.ChannelName, base64.RawURLEncoding.EncodeToString(hash))
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
						ChannelName: channel.GetName(),
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
					bh, err := bcgo.HashProtobuf(b)
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

		if err := bcgo.Update(channel, cache, network, hash, block); err != nil {
			log.Println(err)
			// return - Must send head reference back
		}

		// Reply with current head
		if err := bcgo.WriteDelimitedProtobuf(writer, &bcgo.Reference{
			Timestamp:   channel.GetTimestamp(),
			ChannelName: channel.GetName(),
			BlockHash:   channel.GetHead(),
		}); err != nil {
			log.Println(err)
			return
		}
	}
}
