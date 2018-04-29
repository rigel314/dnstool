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
	Servers []string
	Hosts []host
	Cnames []cname
	NXoverride []string
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
var cfg = config{Servers: []string{"8.8.8.8", "8.8.4.4"}, Hosts: []host{{IP: "127.0.0.1", Name: "example"}}, Cnames: []cname{{Name: "aoeu", Cname: "example"}}, NXoverride: []string{"example.com"}}

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

	conn, err := net.ListenPacket("udp4", "127.0.0.1:53")
	if(err != nil) {
		log.Fatal("Couldn't listen on 127.0.0.1:53")
	}
	defer conn.Close()

	responseCh := make(chan dnsResp)
	defer func() {
		close(responseCh)
	}()
	go func() {
		for {
			resp, ok := <- responseCh
			if(!ok){
				return
			}

			// destAddr := net.Addr(&resp.dest)
			_, err := conn.WriteTo(resp.data, resp.dest)
			if(err != nil) {
				log.Print(err)
				return;
			}
		}
	}()

	fmt.Printf("Server up\n")

	for {
		buf := make([]byte, 1024)

		n, addr, err := conn.ReadFrom(buf)
		if(err != nil) {
			log.Print(err)
			continue
		}

		go func() {
			log.Printf("got %d bytes from %s", n, addr)

			// TODO: inspect request and immediatly send CNAME/"hosts file" reply from configuration

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

			rbuf := make([]byte, 1024)

			rconn.SetReadDeadline(time.Now().Add(time.Second)) // TODO: configurable timeout
			n, _, err := rconn.ReadFrom(rbuf)
			if(err != nil) {
				// TODO: actually, send an NXdomain on timeout
				log.Print(err)
				return
			}

			// TODO: parse response for NXdomain override

			resp := dnsResp{dest: addr, data: rbuf[:n]}
			responseCh <- resp
		}()
	}
}
