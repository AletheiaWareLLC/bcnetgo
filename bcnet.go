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
	"fmt"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/golang/protobuf/proto"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path"
	"time"
)

func Bind(port int, handler func(net.Conn)) {
	address := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Println("Error listening", err)
		return
	}
	defer l.Close()
	log.Println("Listening on" + address)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Error accepting", err)
			return
		}
		go handler(conn)
	}
}

func HTTPSRedirect(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += "?" + r.URL.RawQuery
	}
	log.Println("Redirecting to", target)
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func GetSecurityStore() (string, error) {
	store, ok := os.LookupEnv("SECURITYSTORE")
	if !ok {
		u, err := user.Current()
		if err != nil {
			return "", err
		}
		store = path.Join(u.HomeDir, "bc")
	}
	return store, nil
}

func SetupLogging() (*os.File, error) {
	store, ok := os.LookupEnv("LOGSTORE")
	if !ok {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}
		store = path.Join(u.HomeDir, "bc", "logs")
	}
	if err := os.MkdirAll(store, os.ModePerm); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(path.Join(store, time.Now().Format(time.RFC3339)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return logFile, nil
}

type KeyStore struct {
	Keys map[string]*bcgo.KeyShare
}

func HandleAlias(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	aliases, err := aliasgo.OpenAliasChannel()
	if err != nil {
		log.Println(err)
		return
	}
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var alias string
		if results, ok := query["alias"]; ok && len(results) == 1 {
			alias = results[0]
		}
		log.Println("Alias", alias)
		r, a, err := aliasgo.GetAliasRecord(aliases, alias)
		if err != nil {
			log.Println(err)
			return
		}
		t, err := template.ParseFiles("html/template/alias.html")
		if err != nil {
			log.Println(err)
			return
		}
		data := struct {
			Alias     string
			Timestamp string
			PublicKey string
		}{
			Alias:     alias,
			Timestamp: bcgo.TimestampToString(r.Timestamp),
			PublicKey: base64.RawURLEncoding.EncodeToString(a.PublicKey),
		}
		log.Println("Data", data)
		err = t.Execute(w, data)
		if err != nil {
			log.Println(err)
			return
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func HandleAliasRegister(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	aliases, err := aliasgo.OpenAliasChannel()
	if err != nil {
		log.Println(err)
		return
	}
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var alias string
		if results, ok := query["alias"]; ok && len(results) == 1 {
			alias = results[0]
		}
		log.Println("Alias", alias)
		if err := aliasgo.UniqueAlias(aliases, alias); err != nil {
			log.Println(err)
			return
		}
		var publicKey string
		if results, ok := query["publicKey"]; ok && len(results) == 1 {
			publicKey = results[0]
		}
		log.Println("PublicKey", publicKey)
		t, err := template.ParseFiles("html/template/alias-register.html")
		if err != nil {
			log.Println(err)
			return
		}
		data := struct {
			Alias     string
			PublicKey string
		}{
			Alias:     alias,
			PublicKey: publicKey,
		}
		log.Println("Data", data)
		err = t.Execute(w, data)
		if err != nil {
			log.Println(err)
			return
		}
	case "POST":
		r.ParseForm()
		log.Println("Request", r)
		alias := r.Form["alias"]
		log.Println("Alias", alias)
		publicKey := r.Form["publicKey"]
		log.Println("PublicKey", publicKey)
		publicKeyFormat := r.Form["publicKeyFormat"]
		log.Println("PublicKeyFormat", publicKeyFormat)
		signature := r.Form["signature"]
		log.Println("Signature", signature)
		signatureAlgorithm := r.Form["signatureAlgorithm"]
		log.Println("SignatureAlgorithm", signatureAlgorithm)

		if len(alias) > 0 && len(publicKey) > 0 && len(publicKeyFormat) > 0 && len(signature) > 0 && len(signatureAlgorithm) > 0 {
			if alias[0] == "" {
				log.Println("Empty Alias")
				return
			}

			if err := aliasgo.UniqueAlias(aliases, alias[0]); err != nil {
				log.Println(err)
				return
			}

			pubKey, err := base64.RawURLEncoding.DecodeString(publicKey[0])
			if err != nil {
				log.Println(err)
				return
			}

			pubFormatValue, ok := bcgo.PublicKeyFormat_value[publicKeyFormat[0]]
			if !ok {
				log.Println("Unrecognized Public Key Format")
				return
			}
			pubFormat := bcgo.PublicKeyFormat(pubFormatValue)

			sig, err := base64.RawURLEncoding.DecodeString(signature[0])
			if err != nil {
				log.Println(err)
				return
			}

			sigAlgValue, ok := bcgo.SignatureAlgorithm_value[signatureAlgorithm[0]]
			if !ok {
				log.Println("Unrecognized Signature")
				return
			}
			sigAlg := bcgo.SignatureAlgorithm(sigAlgValue)

			record, err := aliasgo.CreateAliasRecord(alias[0], pubKey, pubFormat, sig, sigAlg)
			if err != nil {
				log.Println(err)
				return
			}

			data, err := proto.Marshal(record)
			if err != nil {
				log.Println(err)
				return
			}

			entries := [1]*bcgo.BlockEntry{
				&bcgo.BlockEntry{
					RecordHash: bcgo.Hash(data),
					Record:     record,
				},
			}

			node, err := bcgo.GetNode()
			if err != nil {
				log.Println(err)
				return
			}

			// Mine record into blockchain
			hash, block, err := node.MineRecords(aliases, entries[:])
			if err != nil {
				log.Println(err)
				return
			}
			if err := aliases.Cast(hash, block); err != nil {
				log.Println(err)
				return
			}
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func HandleBlock(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var channel string
		if results, ok := query["channel"]; ok && len(results) == 1 {
			channel = results[0]
		}
		log.Println("Channel", channel)
		var hash string
		if results, ok := query["hash"]; ok && len(results) == 1 {
			hash = results[0]
		}
		log.Println("Hash", hash)
		if len(channel) > 0 && len(hash) > 0 {
			c, err := bcgo.OpenChannel(channel)
			if err != nil {
				log.Println(err)
				return
			}
			hashBytes, err := base64.RawURLEncoding.DecodeString(hash)
			if err != nil {
				log.Println(err)
				return
			}
			// Read from cache
			block, err := bcgo.ReadBlockFile(c.Cache, hashBytes)
			if err != nil {
				log.Println(err)
				return
			}
			t, err := template.ParseFiles("html/template/block.html")
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
					Payload:              base64.RawURLEncoding.EncodeToString(e.Record.Payload),
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
			log.Println("Data", data)
			err = t.Execute(w, data)
			if err != nil {
				log.Println(err)
				return
			}
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func HandleChannel(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var channel string
		if results, ok := query["channel"]; ok && len(results) == 1 {
			channel = results[0]
		}
		log.Println("Channel", channel)
		if len(channel) > 0 {
			c, err := bcgo.OpenChannel(channel)
			if err != nil {
				log.Println(err)
				return
			}
			// Read from cache
			reference, err := bcgo.ReadHeadFile(c.Cache, c.Name)
			if err != nil {
				log.Println(err)
				return
			}
			t, err := template.ParseFiles("html/template/channel.html")
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
			log.Println("Data", data)
			err = t.Execute(w, data)
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			t, err := template.ParseFiles("html/template/channel-list.html")
			if err != nil {
				log.Println(err)
				return
			}
			type TemplateChannel struct {
				Name      string
				Timestamp string
				Hash      string
			}
			channels := make([]TemplateChannel, 0)
			for name, channel := range bcgo.Channels {
				// Read from cache
				reference, err := bcgo.ReadHeadFile(channel.Cache, name)
				if err != nil {
					log.Println(err)
				} else {
					channels = append(channels, TemplateChannel{
						Name:      name,
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
			log.Println("Data", data)
			err = t.Execute(w, data)
			if err != nil {
				log.Println(err)
				return
			}
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func (ks *KeyStore) HandleKeys(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	switch r.Method {
	case "GET":
		query := r.URL.Query()
		var alias string
		if ts, ok := query["alias"]; ok && len(ts) > 0 {
			alias = ts[0]
		}
		log.Println("Alias", alias)
		if k, ok := ks.Keys[alias]; ok {
			data, err := proto.Marshal(k)
			if err != nil {
				log.Println(err)
				return
			}
			count, err := w.Write(data)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Wrote KeyShare", count, "bytes")
		}
	case "POST":
		r.ParseForm()
		log.Println("Request", r)
		alias := r.Form["alias"]
		log.Println("Alias", alias)
		publicKey := r.Form["publicKey"]
		log.Println("PublicKey", publicKey)
		publicKeyFormat := r.Form["publicKeyFormat"]
		log.Println("PublicKeyFormat", publicKeyFormat)
		privateKey := r.Form["privateKey"]
		log.Println("PrivateKey", privateKey)
		privateKeyFormat := r.Form["privateKeyFormat"]
		log.Println("PrivateKeyFormat", privateKeyFormat)
		password := r.Form["password"]
		log.Println("Password", password)

		if len(alias) > 0 && len(publicKey) > 0 && len(publicKeyFormat) > 0 && len(privateKey) > 0 && len(privateKeyFormat) > 0 && len(password) > 0 {
			a := alias[0]
			publicKey, err := base64.RawURLEncoding.DecodeString(publicKey[0])
			if err != nil {
				log.Println(err)
				return
			}
			pubFormatValue, ok := bcgo.PublicKeyFormat_value[publicKeyFormat[0]]
			if !ok {
				log.Println("Unrecognized Public Key Format")
				return
			}
			pubFormat := bcgo.PublicKeyFormat(pubFormatValue)
			privateKey, err := base64.RawURLEncoding.DecodeString(privateKey[0])
			if err != nil {
				log.Println(err)
				return
			}
			privFormatValue, ok := bcgo.PrivateKeyFormat_value[privateKeyFormat[0]]
			if !ok {
				log.Println("Unrecognized Private Key Format")
				return
			}
			privFormat := bcgo.PrivateKeyFormat(privFormatValue)
			password, err := base64.RawURLEncoding.DecodeString(password[0])
			if err != nil {
				log.Println(err)
				return
			}
			ks.Keys[a] = &bcgo.KeyShare{
				Alias:         a,
				PublicKey:     publicKey,
				PublicFormat:  pubFormat,
				PrivateKey:    privateKey,
				PrivateFormat: privFormat,
				Password:      password,
			}
			go func() {
				// Delete mapping after 2 minutes
				time.Sleep(2 * time.Minute)
				log.Println("Expiring Keys", a)
				delete(ks.Keys, a)
			}()
		}
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func HandleStatic(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, path.Join("html/static", r.URL.Path))
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func HandleBlockPort(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	request, err := bcgo.ReadReference(reader)
	if err != nil {
		log.Println(err)
		return
	}
	blockHash := base64.RawURLEncoding.EncodeToString(request.BlockHash)
	recordHash := base64.RawURLEncoding.EncodeToString(request.RecordHash)
	log.Println("Block Request", conn.RemoteAddr(), request.ChannelName, blockHash, recordHash)
	channel := request.ChannelName
	c, err := bcgo.OpenChannel(channel)
	if err != nil {
		log.Println(err)
		return
	}
	hash := request.BlockHash
	if hash != nil && len(hash) > 0 {
		// Read from cache
		block, err := bcgo.ReadBlockFile(c.Cache, hash)
		if err != nil {
			log.Println(err)
			return
		}
		// Write to connection
		log.Println("Writing block")
		if err := bcgo.WriteBlock(writer, block); err != nil {
			log.Println(err)
			return
		}
	} else {
		hash := request.RecordHash
		if hash != nil && len(hash) > 0 {
			// Search through chain until record hash is found, and return the containing block
			b := c.HeadBlock
			for b != nil {
				for _, e := range b.Entry {
					if bytes.Equal(e.RecordHash, hash) {
						log.Println("Found record, writing block")
						// Write to connection
						if err := bcgo.WriteBlock(writer, b); err != nil {
							log.Println(err)
							return
						}
						return
					}
				}
				h := b.Previous
				if h != nil && len(h) > 0 {
					b, err = bcgo.ReadBlockFile(c.Cache, h)
					if err != nil {
						log.Println(err)
						return
					}
				} else {
					b = nil
				}
			}
		} else {
			log.Println("Missing block hash and record hash")
			return
		}
	}
}

func HandleHeadPort(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	request, err := bcgo.ReadReference(reader)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Head Request", conn.RemoteAddr(), request.ChannelName)
	channel := request.ChannelName
	c, err := bcgo.OpenChannel(channel)
	if err != nil {
		log.Println(err)
		return
	}
	reference, err := bcgo.ReadHeadFile(c.Cache, channel)
	if err != nil {
		log.Println(err)
		return
	}
	blockHash := base64.RawURLEncoding.EncodeToString(reference.BlockHash)
	log.Println("Head Response", reference.ChannelName, blockHash)
	if err := bcgo.WriteReference(writer, reference); err != nil {
		log.Println(err)
		return
	}
}

func HandleCastPort(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	block, err := bcgo.ReadBlock(reader)
	if err != nil {
		log.Println(err)
		return
	}
	// TODO only accept casts from registered customers
	// TODO only accept casts in allowed channels
	data, err := proto.Marshal(block)
	if err != nil {
		log.Println(err)
		return
	}
	channel := block.ChannelName
	hash := bcgo.Hash(data)
	log.Println("Block Cast", conn.RemoteAddr(), channel, base64.RawURLEncoding.EncodeToString(hash))
	c, err := bcgo.OpenChannel(channel)
	if err != nil {
		log.Println(err)
		return
	}
	if err := c.Update(hash, block); err != nil {
		log.Println(err)
		// return - Must send head reference back
	}
	if c.HeadBlock != nil {
		reference := &bcgo.Reference{
			Timestamp:   c.HeadBlock.Timestamp,
			ChannelName: c.Name,
			BlockHash:   c.HeadHash,
		}
		// Reply with current head
		if err := bcgo.WriteReference(writer, reference); err != nil {
			log.Println(err)
			return
		}
	}
}
