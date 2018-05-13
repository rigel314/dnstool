package main

import (
	"os"
	"log"
	"fmt"
	"strings"
	"io/ioutil"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	f, err := os.Create("config.go")
	if(err != nil) {
		log.Println(err)
		log.Fatal("couldn't create config.go")
	}
	defer f.Close()

	fmt.Fprintf(f, "%s\n", "package main")

	cfgFp, err := os.Open("config.json")
	if(err != nil) {
		log.Println(err)
		log.Fatal("failed to open config.json")
	}

	data, err := ioutil.ReadAll(cfgFp)
	if(err != nil) {
		log.Println(err)
		log.Fatal("Failed read of config.json")
	}

	str := strings.Replace(string(data), "\"", "\\\"", -1)
	str = strings.Replace(str, "\n", "", -1)
	_, err = f.Write([]byte("var defaultjs = []byte(\"" + str + "\")\n"))
	if(err != nil) {
		log.Println(err)
		log.Fatal("failed writing config.json")
	}
}
