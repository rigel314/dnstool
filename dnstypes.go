package main

import (
	// "fmt"
	"encoding/binary"
	// "encoding/hex"
)

const headerSize = 12

type dnsQuery struct {
	id uint16
	name string
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
