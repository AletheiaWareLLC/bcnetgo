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
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/golang/protobuf/proto"
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

type KeyStore struct {
	Keys map[string]*bcgo.KeyShare
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

func HandleBlock(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	request, err := bcgo.ReadReference(reader)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Block Request", request)
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

func HandleHead(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	request, err := bcgo.ReadReference(reader)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Head Request", request)
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
	log.Println("Head Response", reference)
	if err := bcgo.WriteReference(writer, reference); err != nil {
		log.Println(err)
		return
	}
}
