package main

import (
	"log"

	"go.accessory.serial-driver/accessory"
)

func main() {
	hvm := accessory.NewHeptaValveMini()
	err := hvm.Connect()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(hvm)

}
