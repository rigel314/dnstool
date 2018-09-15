package main

// A local DNS server with basic filters
// You can configure a list of DNS servers to forward to
// You can configure a hosts file
// You can configure automatic CNAME responses
// You can configure NXdomain overrides for stupid ISPs that intercept and replace unencrypted NXdomain responses with their own bullshit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type config struct {
	General         genCfg
	HTTPlistenPorts []uint16
	Servers         []string
	Hosts           []host
	Cnames          []cname
	Redirect301s    []redirect
	NXoverride      []string
}
type genCfg struct {
	BindIP     string
	DNSPort    int16
	DNSTCPalso bool
	TimeoutMs  int
	ShowStats  bool
}
type host struct {
	IP   string
	Name string
}
type cname struct {
	Name  string
	Cname string
}
type redirect struct {
	From string
	To   string
}

type statistics struct {
	ServerHits []int
}

type dnsResp struct {
	dest net.Addr
	data []byte
	idx  int
}

// Defaults
//go:generate go run gen/defaults.go
var cfg config

var stats statistics

var GIT_VER string = "Uncontrolled"

func main() {
	// Print line numbers
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("GIT_VER: " + GIT_VER)

	// Use all cores if you have to
	runtime.GOMAXPROCS(runtime.NumCPU())

	// load hard coded defaults
	err := json.Unmarshal(defaultjs, &cfg)
	if err != nil {
		log.Println(err)
		log.Fatal("hard coded defaults caused error")
	}
	// load JSON config
	cfgfp, err := os.Open("config.json")
	if err != nil {
		log.Print("config.json failed loading, using defaults")
	} else {
		cfgjs := json.NewDecoder(cfgfp)
		err = cfgjs.Decode(&cfg)
		if err != nil {
			log.Print(err)
			log.Fatal("invalid config") // Die if a parsing error happens
		}
	}

	if cfg.General.DNSTCPalso {
		log.Println("ignoring unsupported config: DNSTCPalso = true")
	}

	// filter out 127.0.0.1's from Server list
	servfilt := cfg.Servers[:0]
	for _, serv := range cfg.Servers {
		if bytes.Compare(net.ParseIP(serv), net.IPv4(127, 0, 0, 1)) != 0 {
			servfilt = append(servfilt, serv)
		}
	}
	cfg.Servers = servfilt

	// initialize statistics
	stats.ServerHits = make([]int, len(cfg.Servers))

	// DNS Server
	bindstr := fmt.Sprintf("%s:%d", cfg.General.BindIP, cfg.General.DNSPort)
	conn, err := net.ListenPacket("udp4", bindstr)
	if err != nil {
		log.Fatal("Couldn't listen on" + bindstr)
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
			resp, ok := <-responseCh
			if !ok {
				log.Print(err)
				continue
			}

			// Actually send response
			_, err := conn.WriteTo(resp.data, resp.dest)
			if err != nil {
				log.Print(err)
				continue
			}
		}
	}()

	// HTTP Servers for serving configured 301 redirects
	for _, port := range cfg.HTTPlistenPorts {
		go func(port uint16) {
			http.HandleFunc("/", redirector)
			err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
			if err != nil {
				log.Fatal(err)
			}
		}(port)
	}

	fmt.Printf("Server up\n")

	for {
		// Create buffer to hold request
		buf := make([]byte, 1024)

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Print(err)
			continue
		}

		// Handle the request
		go func(addr net.Addr, buf []byte) {
			// log.Printf("got %d bytes from %s", n, addr)

			// inspect request and immediately send CNAME/"hosts file" reply from configuration
			q, ok := parseQuery(buf)
			if ok {
				// log.Print(q.name)
				for _, name := range cfg.Cnames {
					if strings.EqualFold(name.Name, q.name) {
						// TODO: make CNAME replies also provide an A answer
						responseCh <- dnsResp{dest: addr, data: genResponse(dnsResponse{id: q.id, name: q.name, cname: name.Cname, a: false})}
						return
					}
				}
				for _, name := range cfg.Hosts {
					if strings.EqualFold(name.Name, q.name) {
						responseCh <- dnsResp{dest: addr, data: genResponse(dnsResponse{id: q.id, name: q.name, ip: name.IP, a: true})}
						return
					}
				}
			}

			fwChan := make(chan dnsResp)

			// Wait for responses from forwarded DNS requests
			go func() {
				select {
				case val, ok := <-fwChan: // Only care about the first reply
					if ok {
						// log.Printf("using dns from server %d\n", val.idx)
						responseCh <- val           // send response to response handler
						stats.ServerHits[val.idx]++ // Update stats
						// Print out stats every 100 forwarded requests
						if cfg.General.ShowStats && sumSlice(stats.ServerHits)%100 == 0 {
							log.Printf("DNS breakdown:\n")
							log.Printf("\t#responses\tserver\n")
							for i, v := range stats.ServerHits {
								log.Printf("\t% 10d\t%s", v, cfg.Servers[i])
							}
						}
					}
				case <-time.After(time.Duration(cfg.General.TimeoutMs) * time.Millisecond):
					log.Println("timout")
				}
			}()

			// Forward request to each configured server
			for idx, srv := range cfg.Servers {
				// launch a goroutine to send requests in parallel
				go func(idx int, srv string) {
					// Make independent socket
					rconn, err := net.ListenPacket("udp4", "")
					if err != nil {
						log.Print(err)
						return
					}
					defer rconn.Close()

					// Send actual request
					remoteAddr := net.UDPAddr{IP: net.ParseIP(srv), Port: 53}
					_, err = rconn.WriteTo(buf[:n], net.Addr(&remoteAddr))
					if err != nil {
						log.Print(err)
						return
					}

					// Create buffer to hold response
					rbuf := make([]byte, 1024)

					rconn.SetReadDeadline(time.Now().Add(time.Duration(cfg.General.TimeoutMs) * time.Millisecond))
					n, _, err := rconn.ReadFrom(rbuf)
					if err != nil {
						log.Printf("err forwarding to %s", srv)
						// TODO: actually, send an NXdomain on timeout
						log.Print(err)
						return
					}

					// TODO: parse response for NXdomain override

					// Create response and copy to smaller buffer
					sm_rbuf := make([]byte, n)
					copy(sm_rbuf, rbuf[:n])
					resp := dnsResp{dest: addr, data: sm_rbuf, idx: idx}

					// Send response to client
					// non-blocking because we only care about the fastest response
					select {
					case fwChan <- resp:
					default:
						// log.Printf("chan %d not ready\n", idx)
					}
				}(idx, srv)
			}
		}(addr, buf)
	}
}

// Serves 301 redirects, if configured
func redirector(w http.ResponseWriter, req *http.Request) {
	dest := req.Host + req.URL.String()
	log.Println(req.Host)
	for _, name := range cfg.Redirect301s {
		if strings.EqualFold(name.From, req.Host) {
			newdest := "http://" + name.To + req.URL.String()
			log.Printf("redirecting %s=>%s\n", dest, newdest)
			http.Redirect(w, req, newdest, http.StatusMovedPermanently)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, fmt.Sprintf("no redirect or reverse proxy config for %s\n", req.Host))
}

// Sum the elements of a slice of ints
func sumSlice(s []int) int {
	var sum int = 0
	for _, v := range s {
		sum += v
	}

	return sum
}
