package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"go.accessory.serial-driver/accessory"
)

func main() {
	hvm := accessory.NewHeptaValveMini()
	err := hvm.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer hvm.FirmataAdaptor.Disconnect()
	log.Printf("%#v", hvm.FirmataAdaptor)

	for i, pin := range hvm.FirmataAdaptor.Board.Pins() {
		log.Println(i, pin.Mode, pin.State, pin.SupportedModes, pin.Value)
	}

	hvm.Configure()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter <valve>: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		log.Printf("You entered %s (%d)", text, len(text))
		valveStr := text

		valve, err := strconv.Atoi(valveStr)
		if err != nil {
			log.Println(err)
			continue
		}

		err = hvm.SwitchToValve(valve)
		if err != nil {
			log.Println(err)
			continue
		}

	}

}
