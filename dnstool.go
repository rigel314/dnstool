package main

// A local DNS server with basic filters
// You can configure a list of DNS servers to forward to
// You can configure a hosts file
// You can configure automatic CNAME responses
// You can configure NXdomain overrides for stupid ISPs that intercept and replace unencrypted NXdomain responses with their own bullshit

import (
	"fmt"
	"log"
	"net"
	"time"
	"os"
	"encoding/json"
)

type config struct {
	General genCfg
	Servers []string
	Hosts []host
	Cnames []cname
	NXoverride []string
}
type genCfg struct {
	BindIP string
	Port int16
	TCPalso bool
	TimeoutMs int
}
type host struct {
	IP string
	Name string
}
type cname struct {
	Name string
	Cname string
}

type dnsResp struct {
	dest net.Addr
	data []byte
}

// Defaults
var cfg = config{General: genCfg{BindIP: "127.0.0.1", Port: 53, TCPalso: false, TimeoutMs: 1000}, Servers: []string{"8.8.8.8", "8.8.4.4"}, Hosts: []host{{IP: "127.0.0.1", Name: "example"}}, Cnames: []cname{{Name: "aoeu", Cname: "example"}}, NXoverride: []string{"example.com"}}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// load JSON config
	cfgfp, err := os.Open("config.json")
	if(err != nil) {
		log.Print("config.json failed loading")
	} else {
		cfgjs := json.NewDecoder(cfgfp)
		err = cfgjs.Decode(&cfg)
		if(err != nil) {
			log.Print(err)
			log.Fatal("invalid config")
		}
	}

	// fmt.Println(cfg)

	// Server
	conn, err := net.ListenPacket("udp4", "127.0.0.1:53") // TODO: config
	if(err != nil) {
		log.Fatal("Couldn't listen on 127.0.0.1:53")
	}
	defer conn.Close()

	// channel to hold responses
	responseCh := make(chan dnsResp)
	defer func() {
		close(responseCh)
	}()
	// Response sender
	go func() {
		for {
			// wait for response to be generated
			resp, ok := <- responseCh
			if(!ok) {
				log.Print(err)
				continue
			}

			// Actually send response
			_, err := conn.WriteTo(resp.data, resp.dest)
			if(err != nil) {
				log.Print(err)
				continue
			}
		}
	}()

	fmt.Printf("Server up\n")

	for {
		// Create buffer to hold request
		buf := make([]byte, 1024)

		n, addr, err := conn.ReadFrom(buf)
		if(err != nil) {
			log.Print(err)
			continue
		}

		// Handle the request
		go func() {
			log.Printf("got %d bytes from %s", n, addr)

			// TODO: inspect request and immediatly send CNAME/"hosts file" reply from configuration

			// Forward request to each configured server
			rconn, err := net.ListenPacket("udp4", "")
			if(err != nil) {
				log.Print(err)
				return
			}
			defer rconn.Close()

			remoteAddr := net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 53} // TOOD: configurable dns forwarding (a list)
			_, err = rconn.WriteTo(buf[:n], net.Addr(&remoteAddr))
			if(err != nil) {
				log.Print(err)
				return
			}

			// Create buffer to hold response
			rbuf := make([]byte, 1024)

			rconn.SetReadDeadline(time.Now().Add(time.Second)) // TODO: configurable timeout
			n, _, err := rconn.ReadFrom(rbuf)
			if(err != nil) {
				// TODO: actually, send an NXdomain on timeout
				log.Print(err)
				return
			}

			// TODO: parse response for NXdomain override

			// Send response to client
			resp := dnsResp{dest: addr, data: rbuf[:n]}
			responseCh <- resp
		}()
	}
}
