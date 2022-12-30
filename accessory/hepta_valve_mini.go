package accessory

import (
	"errors"
	"fmt"
	"log"

	"go.bug.st/serial/enumerator"
	"gobot.io/x/gobot/platforms/firmata"
	"gobot.io/x/gobot/platforms/firmata/client"
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
	0 + 18,
	1 + 18,
	2 + 18,
	4 + 18,
}

type HeptaValveMini struct {
	PortName       string
	Header         *EEPROMHeader
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

		hvm.Header = &eepromHeader
		hvm.FirmataAdaptor = firmataAdaptor
	}
	return err
}

func (hvm *HeptaValveMini) Configure() error {
	var err error
	for i, valvePin := range MINI_VALVE_GPIO_CFG {

		ledPin := MINI_LED_GPIO_CFG[i]
		err = hvm.FirmataAdaptor.Board.SetPinMode(valvePin, client.Output)
		if err != nil {
			return err
		}
		err = hvm.FirmataAdaptor.Board.SetPinMode(ledPin, client.Output)
		if err != nil {
			return err
		}
		var level = 0
		if i == 0 {
			level = 1
		}

		hvm.FirmataAdaptor.Board.DigitalWrite(valvePin, level)
		if err != nil {
			return err
		}
		hvm.FirmataAdaptor.Board.DigitalWrite(ledPin, level)
		if err != nil {
			return err
		}
	}

	hvm.SetPwrLed(1)

	return err
}

func (hvm *HeptaValveMini) SetPwrLed(level byte) error {
	var err error
	pInt := MINI_PWR_GPIO_CFG
	pin := fmt.Sprint(pInt)

	log.Printf("Setting PWR LED (pin %s) to %d", pin, level)

	err = hvm.FirmataAdaptor.DigitalWrite(pin, level)
	return err
}

func (hvm *HeptaValveMini) SetValve(valveNb int, level byte) error {
	var err error

	err = checkValveNumber(valveNb)
	if err != nil {
		return err
	}
	pInt := MINI_VALVE_GPIO_CFG[valveNb]
	pin := fmt.Sprint(pInt)

	err = hvm.FirmataAdaptor.DigitalWrite(pin, level)
	return err
}

func (hvm *HeptaValveMini) SetLED(valveNb int, level byte) error {
	var err error

	err = checkValveNumber(valveNb)
	if err != nil {
		return err
	}
	pInt := MINI_LED_GPIO_CFG[valveNb]
	pin := fmt.Sprint(pInt)

	err = hvm.FirmataAdaptor.DigitalWrite(pin, level)
	return err
}

func (hvm *HeptaValveMini) SetValveAndLED(valveNb int, level byte) error {
	var err error
	err = hvm.SetValve(valveNb, level)
	if err != nil {
		return err
	}
	err = hvm.SetLED(valveNb, level)
	if err != nil {
		return err
	}
	return err
}

func (hvm *HeptaValveMini) SwitchToValve(valveNb int) error {
	var err error
	err = hvm.SetValveAndLED(valveNb, 1)
	if err != nil {
		return err
	}
	for i := 0; i < hvm.Header.NbValves; i++ {
		if i == valveNb {
			continue
		}
		err = hvm.SetValveAndLED(i, 0)
		if err != nil {
			return err
		}
	}
	return err
}

func checkValveNumber(valveNb int) error {
	var err error
	if valveNb > len(MINI_VALVE_GPIO_CFG)-1 {
		err = fmt.Errorf("heptaValveMini valveNb overflow: %d (max idx = %d)", valveNb, len(MINI_LED_GPIO_CFG)-1)
	}
	return err
}
