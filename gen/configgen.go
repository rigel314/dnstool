package main

// generates parameter struct from defalt inifile

import (
	"fmt"
	"log"
	"os"
	"strings"
	"gopkg.in/go-ini/ini.v1"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := ini.Load("./config/dnstool.ini")
	if(err != nil) {
		log.Print(err)
		log.Fatal("ini missing")
	}

	fp, err := os.Create("./config.go")
	if(err != nil) {
		log.Print(err)
		log.Fatal("failed to create output file")
	}

	fmt.Print("Writing config struct...\n")
	fmt.Fprintf(fp, "%s", "package main\n\ntype configDataDNStool struct {\n")
	for _, key := range cfg.Section("").Keys() {
		typename := strings.Split(key.Name(), " ")
		// fmt.Printf("%d: %s(default: %s)\n", i, key.Name(), key.Value())
		fmt.Fprintf(fp, "\t%s %s\n", typename[1], typename[0])
	}
	fmt.Fprintf(fp, "}\n")
}
