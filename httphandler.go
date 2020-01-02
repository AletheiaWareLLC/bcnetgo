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
	"encoding/base64"
	"fmt"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/netgo"
	"html/template"
	"log"
	"net/http"
)

func BlockHandler(cache bcgo.Cache, network bcgo.Network, template *template.Template) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
		switch r.Method {
		case "GET":
			channel := netgo.GetQueryParameter(r.URL.Query(), "channel")
			log.Println("Channel", channel)

			hash := netgo.GetQueryParameter(r.URL.Query(), "hash")
			log.Println("Hash", hash)

			if len(channel) > 0 && len(hash) > 0 {
				hashBytes, err := base64.RawURLEncoding.DecodeString(hash)
				if err != nil {
					log.Println(err)
					return
				}
				// Read block
				block, err := bcgo.GetBlock(channel, cache, network, hashBytes)
				if err != nil {
					log.Println(err)
					return
				}
				type TemplateReference struct {
					Timestamp  string
					Channel    string
					BlockHash  string
					RecordHash string
				}
				type TemplateAccess struct {
					Alias               string
					SecretKey           string
					EncryptionAlgorithm string
				}
				type TemplateEntry struct {
					Hash                 string
					Timestamp            string
					Creator              string
					Access               []TemplateAccess
					Payload              string
					CompressionAlgorithm string
					EncryptionAlgorithm  string
					Signature            string
					SignatureAlgorithm   string
					Reference            []TemplateReference
				}
				entries := make([]TemplateEntry, 0)
				for _, e := range block.Entry {
					accesses := make([]TemplateAccess, 0)
					for _, a := range e.Record.Access {
						accesses = append(accesses, TemplateAccess{
							Alias:               a.Alias,
							SecretKey:           base64.RawURLEncoding.EncodeToString(a.SecretKey),
							EncryptionAlgorithm: a.EncryptionAlgorithm.String(),
						})
					}
					references := make([]TemplateReference, 0)
					for _, r := range e.Record.Reference {
						references = append(references, TemplateReference{
							Timestamp:  bcgo.TimestampToString(r.Timestamp),
							Channel:    r.ChannelName,
							BlockHash:  base64.RawURLEncoding.EncodeToString(r.BlockHash),
							RecordHash: base64.RawURLEncoding.EncodeToString(r.RecordHash),
						})
					}
					entries = append(entries, TemplateEntry{
						Hash:                 base64.RawURLEncoding.EncodeToString(e.RecordHash),
						Timestamp:            bcgo.TimestampToString(e.Record.Timestamp),
						Creator:              e.Record.Creator,
						Access:               accesses,
						Payload:              base64.RawURLEncoding.EncodeToString(e.Record.Payload), // TODO allow override for custom rendering
						CompressionAlgorithm: e.Record.CompressionAlgorithm.String(),
						EncryptionAlgorithm:  e.Record.EncryptionAlgorithm.String(),
						Signature:            base64.RawURLEncoding.EncodeToString(e.Record.Signature),
						SignatureAlgorithm:   e.Record.SignatureAlgorithm.String(),
						Reference:            references,
					})
				}
				data := struct {
					Hash      string
					Timestamp string
					Channel   string
					Length    string
					Previous  string
					Miner     string
					Nonce     string
					Entry     []TemplateEntry
				}{
					Hash:      hash,
					Timestamp: bcgo.TimestampToString(block.Timestamp),
					Channel:   channel,
					Length:    fmt.Sprintf("%d", block.Length),
					Previous:  base64.RawURLEncoding.EncodeToString(block.Previous),
					Miner:     block.Miner,
					Nonce:     fmt.Sprintf("%d", block.Nonce),
					Entry:     entries,
				}
				if err := template.Execute(w, data); err != nil {
					log.Println(err)
					return
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}

func ChannelHandler(cache bcgo.Cache, network bcgo.Network, template *template.Template) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
		switch r.Method {
		case "GET":
			channel := netgo.GetQueryParameter(r.URL.Query(), "channel")
			log.Println("Channel", channel)
			if len(channel) > 0 {
				reference, err := bcgo.GetHeadReference(channel, cache, network)
				if err != nil {
					log.Println(err)
					return
				}
				data := struct {
					Channel   string
					Timestamp string
					Hash      string
				}{
					Channel:   channel,
					Timestamp: bcgo.TimestampToString(reference.Timestamp),
					Hash:      base64.RawURLEncoding.EncodeToString(reference.BlockHash),
				}
				if err := template.Execute(w, data); err != nil {
					log.Println(err)
					return
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}

func ChannelListHandler(cache bcgo.Cache, network bcgo.Network, template *template.Template, list func() []*bcgo.Channel) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
		switch r.Method {
		case "GET":
			type TemplateChannel struct {
				Name      string
				Timestamp string
				Hash      string
			}
			channels := make([]TemplateChannel, 0)
			for _, channel := range list() {
				reference, err := bcgo.GetHeadReference(channel.GetName(), cache, network)
				if err != nil {
					log.Println(err)
				} else {
					channels = append(channels, TemplateChannel{
						Name:      channel.GetName(),
						Timestamp: bcgo.TimestampToString(reference.Timestamp),
						Hash:      base64.RawURLEncoding.EncodeToString(reference.BlockHash),
					})
				}
			}
			data := struct {
				Channel []TemplateChannel
			}{
				Channel: channels,
			}
			if err := template.Execute(w, data); err != nil {
				log.Println(err)
				return
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}

func PeriodicValidationHandler(channel *bcgo.Channel, cache bcgo.Cache, network bcgo.Network, template *template.Template) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
		switch r.Method {
		case "GET":
			hash := netgo.GetQueryParameter(r.URL.Query(), "hash")
			log.Println("Hash", hash)
			hashBytes := channel.GetHead()
			var err error
			if len(hash) > 0 {
				hashBytes, err = base64.RawURLEncoding.DecodeString(hash)
				if err != nil {
					log.Println(err)
					return
				}
			}
			block, err := bcgo.GetBlock(channel.GetName(), cache, network, hashBytes)
			if err != nil {
				log.Println(err)
				return
			}
			if block != nil {
				data := struct {
					// TODO
				}{
					// TODO
				}
				if err := template.Execute(w, data); err != nil {
					log.Println(err)
					return
				}
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			log.Println("Unsupported method", r.Method)
		}
	}
}
