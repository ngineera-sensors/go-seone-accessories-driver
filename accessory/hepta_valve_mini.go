package accessory

import (
	"errors"
	"fmt"
	"log"

	"go.bug.st/serial/enumerator"
	"gobot.io/x/gobot/platforms/firmata"
)

var MINI_PWR_GPIO_CFG = 3

var MINI_VALVE_GPIO_CFG = [8]int{
	13,
	10,
	5,
	9,
	12,
	11,
	6,
	4,
}

var MINI_LED_GPIO_CFG = [8]int{
	2,
	0,
	1,
	8,
	0 + 14,
	1 + 14,
	2 + 14,
	4 + 14,
}

type HeptaValveMini struct {
	PortName       string
	FirmataAdaptor *firmata.Adaptor
}

func NewHeptaValveMini() *HeptaValveMini {
	return &HeptaValveMini{}
}

func (hvm *HeptaValveMini) Connect() error {
	var err error

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return err
	}
	for _, port := range ports {
		if port.IsUSB {
			if port.VID == "2341" && port.PID == "8037" {
				hvm.PortName = port.Name
				break
			}
		}
	}
	if hvm.PortName == "" {
		err = errors.New("neose mini board not connected")
		return err
	} else {
		log.Printf("Found Arduino Micro port: %s", hvm.PortName)

		firmataAdaptor := firmata.NewAdaptor(hvm.PortName)
		err = firmataAdaptor.Connect()
		if err != nil {
			return err
		}
		defer firmataAdaptor.Disconnect()

		firmataAdaptor.AddEvent("SysexResponse")

		eepromHeader, err := ReadBoardSysexEEPROMHeader(firmataAdaptor)
		if err != nil {
			log.Fatal(err)
		}

		if eepromHeader.DeviceName == "mini-sampler" {
			log.Printf("Found mini-sampler; NbValves: %d", eepromHeader.NbValves)
		} else {
			log.Printf("Found an accessory but not the mini-sampler: %s", eepromHeader.DeviceName)
		}

		hvm.FirmataAdaptor = firmataAdaptor
	}
	return err
}

func (hvm *HeptaValveMini) SetValve(valveNb int, level byte) error {
	var err error

	err = checkValveNumber(valveNb)
	if err != nil {
		return err
	}

	pin := fmt.Sprint(MINI_VALVE_GPIO_CFG[valveNb])
	err = hvm.FirmataAdaptor.DigitalWrite(pin, level)
	return err
}

func (hvm *HeptaValveMini) ToggleValve(valveNb int) error {
	var err error

	err = checkValveNumber(valveNb)
	if err != nil {
		return err
	}

	pin := fmt.Sprint(MINI_VALVE_GPIO_CFG[valveNb])
	val, err := hvm.FirmataAdaptor.DigitalRead(pin)
	if err != nil {
		return err
	}
	if val == -1 {
		err = fmt.Errorf("error reading current state of valve %d (pin %s)", valveNb, pin)
	}
	level := byte(val ^ 1)
	err = hvm.FirmataAdaptor.DigitalWrite(pin, level)
	return err
}

func checkValveNumber(valveNb int) error {
	var err error
	if valveNb > len(MINI_VALVE_GPIO_CFG)-1 {
		err = fmt.Errorf("heptaValveMini valveNb overflow: %d (max idx = %d)", valveNb, len(MINI_LED_GPIO_CFG)-1)
	}
	return err
}
