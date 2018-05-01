package main

import (
	"fmt"
	"encoding/binary"
	"strings"
	// "encoding/hex"
)

const headerSize = 12

type dnsQuery struct {
	id uint16
	name string
}

type dnsResponse struct {
	id uint16
	name string
	ip string
	cname string
	a bool
}

func parseQuery(bytes []byte) (dnsQuery, bool) {
	qret := dnsQuery{}
	if(len(bytes) < headerSize) {
		return qret, false
	}

	qret.id = binary.BigEndian.Uint16(bytes[0:2])
	flags := binary.BigEndian.Uint16(bytes[2:4])
	qdcount := binary.BigEndian.Uint16(bytes[4:6])
	// ancount := binary.BigEndian.Uint16(bytes[6:8])
	// nscount := binary.BigEndian.Uint16(bytes[8:10])
	// arcount := binary.BigEndian.Uint16(bytes[10:12])

	if(flags & 1<<15 != 0 ||
		flags & 0xF<<11 != 0 ||
		flags & 1<<9 != 0 ||
		qdcount != 1) {
		return qret, false
	}

	ok := false
	qret.name, ok = bytes2name(bytes[12:])
	if(!ok) {
		return qret, false
	}

	return qret, true
}

func genResponse(resp dnsResponse) []byte {
	ret := []byte{}
	l := 0

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], resp.id)
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 1<<15 | 1<<10 | 1<<7)
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 0)
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 1)
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 0)
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 0)
	l += 2

	ret = append(ret, name2bytes(resp.name)...)
	l = len(ret)

	ret = append(ret, []byte{0,0}...)
	if(!resp.a) { // CNAME response
		binary.BigEndian.PutUint16(ret[l:l+2], 5)
	} else { // A response
		binary.BigEndian.PutUint16(ret[l:l+2], 1)
	}
	l += 2

	ret = append(ret, []byte{0,0}...)
	binary.BigEndian.PutUint16(ret[l:l+2], 1)
	l += 2

	ret = append(ret, []byte{0,0,0,0}...)
	binary.BigEndian.PutUint32(ret[l:l+4], 300)
	l += 2

	ret = append(ret, []byte{0,0}...)
	if(!resp.a) { // CNAME response
		b := name2bytes(resp.cname)
		binary.BigEndian.PutUint16(ret[l:l+2], uint16(len(b)))
		ret = append(ret, b...)
	} else { // A response
		binary.BigEndian.PutUint16(ret[l:l+2], 4)
		ret = append(ret, []byte{0,0,0,0}...)
		var a, b, c, d uint8
		fmt.Sscanf(resp.ip, "%d.%d.%d.%d", a, b, c, d)
		ret = append(ret, a, b, c, d)
	}

	return ret
}

func bytes2name(bytes []byte) (string, bool) {
	ret := ""
	l := -1
	for(l != 0) {
		if(len(bytes) < 1) {
			return ret, false
		}
		// fmt.Printf("%d\n", l)
		l = int(bytes[0]) // Should be safe to convert byte to int and never get a negative

		if(len(bytes) < 1+l) {
			return ret, false
		}
		// fmt.Print(hex.Dump(bytes[1:1+l]))
		ret += string(bytes[1:1+l])
		if(l > 0) {
			ret += "."
		}
		bytes = bytes[1+l:]
	}
	return ret, true
}

func name2bytes(name string) []byte {
	ret := []byte{}

	for _, s := range strings.Split(name, ".") {
		ret = append(ret, byte(len(s)))
		ret = append(ret, []byte(s)...)
	}

	return ret
}
