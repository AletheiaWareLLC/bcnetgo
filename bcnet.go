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
	"aletheiaware.com/bcgo"
	"aletheiaware.com/bcgo/network"
	"fmt"
	"log"
	"net"
)

func BindAllTCP(c bcgo.Cache, n *network.TCP, cb func(string) (bcgo.Channel, error)) {
	// Serve Connect Requests
	go BindTCP(network.PORT_CONNECT, ConnectPortTCPHandler(n, func(string, string) bool {
		return true
	}))
	// Serve Block Requests
	go BindTCP(network.PORT_GET_BLOCK, BlockPortTCPHandler(c))
	// Serve Head Requests
	go BindTCP(network.PORT_GET_HEAD, HeadPortTCPHandler(c))
	// Serve Block Updates
	go BindTCP(network.PORT_BROADCAST, BroadcastPortTCPHandler(c, n, cb))
}

func BindTCP(port int, handler func(net.Conn)) {
	address, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Println("Error resolving", err)
		return
	}
	l, err := net.ListenTCP("tcp", address)
	if err != nil {
		log.Println("Error listening", err)
		return
	}
	defer l.Close()
	log.Println("Listening on", address)
	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			log.Println("Error accepting", err)
			return
		}
		go handler(conn)
	}
}
