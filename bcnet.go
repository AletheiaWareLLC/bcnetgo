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
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
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
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	target := "https://" + r.Host + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += "?" + r.URL.RawQuery
	}
	log.Println("Redirecting to", target)
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.Proto, r.Method, r.Host, r.URL.Path)
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, path.Join("html/static", r.URL.Path))
	default:
		log.Println("Unsupported method", r.Method)
	}
}

func GetQueryParameter(query url.Values, parameter string) string {
	if results, ok := query[parameter]; ok && len(results) > 0 {
		return results[0]
	}
	return ""
}
