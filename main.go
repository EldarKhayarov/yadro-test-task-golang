package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	table := &Table{}
	err := table.ParseTable(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(table.String())
}
