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
	"io"
	// "strings"
	"net/http"
	"encoding/json"
)

type config struct {
	General genCfg
	HTTPlistenPorts []uint16
	Servers []string
	Hosts []host
	Cnames []cname
	Redirect301s []redirect
	NXoverride []string
}
type genCfg struct {
	BindIP string
	DNSPort int16
	DNSTCPalso bool
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
type redirect struct {
	From string
	To string
}

type dnsResp struct {
	dest net.Addr
	data []byte
}

// Defaults
var cfg = config{General: genCfg{BindIP: "127.0.0.1", DNSPort: 53, DNSTCPalso: false, TimeoutMs: 1000}, Servers: []string{"8.8.8.8", "8.8.4.4"}, Hosts: []host{{IP: "127.0.0.1", Name: "example"}}, Cnames: []cname{{Name: "aoeu", Cname: "example"}}, NXoverride: []string{"example.com"}}

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
	// Response sender loop
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

	go func() {
		http.HandleFunc("/", redirector)
		http.ListenAndServe("127.0.0.1:80", nil)
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
			q, ok := parseQuery(buf)
			if(ok) {
				log.Print(q.name)
				for _, name := range cfg.Cnames {
					if(name.Name == q.name) {
						responseCh <- dnsResp{dest: addr, data: genResponse(dnsResponse{id: q.id, name: q.name, cname: name.Cname, a: false})}
						return
					}
				}
				for _, name := range cfg.Hosts {
					if(name.Name == q.name) {
						responseCh <- dnsResp{dest: addr, data: genResponse(dnsResponse{id: q.id, name: q.name, ip: name.IP, a: true})}
						return
					}
				}
			}

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

func redirector(w http.ResponseWriter, req *http.Request) {
	dest := req.Host + req.URL.String()
	log.Println(req.Host)
	for _, name := range cfg.Redirect301s {
		if(name.From == req.Host) {
			newdest := "http://" + name.To + req.URL.String()
			log.Printf("redirecting %s=>%s\n", dest, newdest)
			http.Redirect(w, req, newdest, http.StatusMovedPermanently)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, fmt.Sprintf("no redirect or reverse proxy config for %s\n", req.Host))
}
