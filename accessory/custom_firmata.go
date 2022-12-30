package accessory

import (
	"fmt"
	"log"
	"time"

	"gobot.io/x/gobot/platforms/firmata"
)

const (
	// User Sysex commands
	SYSEX_USR_CHECK_CRC       byte = 0 // Check CRC will trigger a CRC status callback; uint8_t msg[1] =  {0}
	SYSEX_USR_WR_EEPROM       byte = 1 // Maximum size is 28; uint8_t msg[X] =  {1, size, indexMSB, indexLSB, data1, …, dataX}
	SYSEX_USR_RD_EEPROM       byte = 2 // Maximum size is 127, read operation will trigger a read EEPROM callback; uint8_t msg[4] =  {2, size, indexMSB, indexLSB}
	SYSEX_USR_CRC_STATUS_CB   byte = 3 // X = 1 CRC valid, X = 0 CRC invalid; Next successful write operation will reset CRC to valid (1); uint8_t msg[1] =  {3, X}
	SYSEX_USR_RD_EEPROM_CB    byte = 4 // uint8_t msg[X] = {4, size, indexMSB, indexLSB, data1, …, dataX}
	SYSEX_USR_WRITE_CMPLTE_CB byte = 5 // uint8_t msg[4] = {5, size, indexMSB, indexLSB}
)

const (
	EEPROM_HEADER_ADDR        = 0
	EEPROM_HEADER_LEN         = 51
	EEPROM_VALVE_NAME_ADDR    = 51
	EEPROM_ONE_VALVE_NAME_LEN = 20
)

const (
	SYSEX_USR_RD_EEPROM_TIMEOUT = 3 * time.Second
	SYSEX_USR_WR_EEPROM_TIMEOUT = 3 * time.Second
)

type SysexResponse struct {
	Command byte
	Data    []byte
}

type EEPROMData struct {
	Size    byte
	Address uint16 // uint16 from [MSB, LSB]
	Payload []byte
}

type EEPROMHeader struct {
	DeviceName       string
	SerialNumber     string
	HardwareRevision string
	NbValves         int
}

func encode7bitPairs(payload []byte) []byte {
	sysexEncodedMsg := make([]byte, 0)

	for _, b := range payload {
		lsb := b & 0x7f
		msb := b >> 7
		sysexEncodedMsg = append(sysexEncodedMsg, lsb, msb)
	}
	return sysexEncodedMsg
}

func decode7bitPairs(sysexEncodedPaylaod []byte) []byte {
	var decodedPayload []byte
	for i := 0; i < len(sysexEncodedPaylaod)-1; i += 2 {
		lsb := sysexEncodedPaylaod[i]
		msb := sysexEncodedPaylaod[i+1]
		value := (lsb & 0x7f) + (msb << 7)
		decodedPayload = append(decodedPayload, value)
	}
	return decodedPayload
}

func decodeEEPROMString(payload []byte) string {
	str := ""
	for _, b := range payload {
		if b == 0 {
			continue
		}
		str += string(b)
	}
	return str
}

func decodeEEPROMHeader(payload []byte) (EEPROMHeader, error) {
	// Device name :   idx 0, len 20
	// Serial number : idx 20, len 10
	// Hardware rev :  idx 30, len 20
	// Nb Valves :     idx 50, len 1

	var err error
	header := EEPROMHeader{}

	if len(payload) < EEPROM_HEADER_LEN {
		err = fmt.Errorf("invalid eeprom header length: %d but %d is required", len(payload), EEPROM_HEADER_LEN)
		return header, err
	}

	header.DeviceName = decodeEEPROMString(payload[0:20])
	header.SerialNumber = decodeEEPROMString(payload[20:30])
	header.HardwareRevision = decodeEEPROMString(payload[30:40])
	header.NbValves = int(payload[50])

	return header, err
}

func parseSysexResponse(s interface{}) (SysexResponse, error) {
	var err error
	var sysexResponse SysexResponse

	var sysexTram []byte
	var ok bool
	if sysexTram, ok = s.([]byte); !ok {
		err = fmt.Errorf("error reading sysex response: casting message to []byte is NOK. Msg: ", sysexTram)
		return sysexResponse, err
	}
	if len(sysexTram) < 4 {
		err = fmt.Errorf("error reading sysex response: response is too short: len = %d", len(sysexTram))
		return sysexResponse, err
	}

	sysexRawPayload := sysexTram[2 : len(sysexTram)-1]
	sysexData := decode7bitPairs(sysexRawPayload)

	sysexResponse = SysexResponse{
		Command: sysexTram[1],
		Data:    sysexData,
	}
	return sysexResponse, err
}

func parseEEPROMData(sysexResponse SysexResponse) (EEPROMData, error) {
	var err error
	eepromData := EEPROMData{
		Size:    sysexResponse.Data[0],
		Address: (uint16(sysexResponse.Data[1]) << 8) | uint16(sysexResponse.Data[2]&0xf), // MSB | LSB
	}

	if len(sysexResponse.Data[3:]) < int(eepromData.Size) {
		err = fmt.Errorf(
			"Sysex response is shorter than size declared in the header: size = %d, actual length: %d", eepromData.Size, len(sysexResponse.Data)-3,
		)
		return eepromData, err
	}

	eepromData.Payload = sysexResponse.Data[3 : 3+eepromData.Size]

	return eepromData, err
}

func ReadBoardSysexEEPROMHeader(c *firmata.Adaptor) (EEPROMHeader, error) {
	var err error
	var eepromHeader EEPROMHeader

	errCh := make(chan error, 1)
	eepromHeaderChan := make(chan EEPROMHeader, 1)

	err = c.Once(c.Event("SysexResponse"), func(s interface{}) {
		log.Println("custom sysex response callback", s)

		sysexResponse, err := parseSysexResponse(s)
		if err != nil {
			errCh <- err
			return
		}

		switch sysexResponse.Command {
		case SYSEX_USR_RD_EEPROM_CB:

			eepromData, err := parseEEPROMData(sysexResponse)

			h, err := decodeEEPROMHeader(eepromData.Payload)
			if err != nil {
				errCh <- err
			}

			eepromHeaderChan <- h
		default:
			log.Printf("SysexResponse CMD not implemented: %v. Ignoring..", sysexResponse.Command)
		}
	})

	var addr uint16 = EEPROM_HEADER_ADDR

	msg := []byte{
		SYSEX_USR_RD_EEPROM,
		EEPROM_HEADER_LEN,
		byte(addr >> 8),   // MSB
		byte(addr & 0xff), // LSB
	}

	sysexEncodedMsg := encode7bitPairs(msg)

	err = c.WriteSysex(sysexEncodedMsg)
	if err != nil {
		return eepromHeader, err
	}

	select {
	case eepromHeader = <-eepromHeaderChan:
		break
	case err = <-errCh:
		break
	case <-time.After(SYSEX_USR_RD_EEPROM_TIMEOUT):
		err = fmt.Errorf("timeout while reading board sysex eeprom data")
		break
	}

	return eepromHeader, err
}
