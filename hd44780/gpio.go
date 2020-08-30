package hd44780

import (
	"errors"
	"time"

	"machine"
)

type GPIO struct {
	dataPins []machine.Pin
	en       machine.Pin
	rw       machine.Pin
	rs       machine.Pin

	write       func(data byte)
	writeNibble func(data byte)
	read        func() byte
}

func newGPIO(dataPins []machine.Pin, en, rs, rw machine.Pin, mode byte) Device {
	pins := make([]machine.Pin, len(dataPins))
	for i := 0; i < len(dataPins); i++ {
		dataPins[i].Configure(machine.PinConfig{Mode: machine.PinOutput})
		pins[i] = dataPins[i]
	}
	en.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rw.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rw.Low()

	gpio := GPIO{
		dataPins: pins,
		en:       en,
		rs:       rs,
		rw:       rw,
	}

	if mode == DATA_LENGTH_4BIT {
		gpio.write = debugWrite(gpio.write4BitMode, false)
		gpio.read = gpio.read4BitMode
		gpio.writeNibble = debugWrite(gpio.write4BitNibble, true)
	} else {
		gpio.write = gpio.write8BitMode
		gpio.read = gpio.read8BitMode
	}

	return Device{
		bus:        &gpio,
		datalength: mode,
	}
}

// SetCommandMode sets command/instruction mode
func (g *GPIO) SetCommandMode(set bool) {
	if set {
		g.rs.Low()
	} else {
		g.rs.High()
	}
}

// Write writes len(data) bytes from data to display driver
func (g *GPIO) Write(data []byte) (n int, err error) {
	g.rw.Low()
	for _, d := range data {
		g.write(d)
		n++
	}
	return n, nil
}

func (g *GPIO) WriteNibble(data []byte) (n int, err error) {
	g.rw.Low()
	for _, d := range data {
		g.writeNibble(d)
		n++
	}
	g.pulseEnable()
	return n, nil
}

func (g *GPIO) pulseEnable() {
	g.en.Low()
	time.Sleep(time.Microsecond)
	g.en.High()
	time.Sleep(time.Microsecond) // enable pulse must be >450ns
	g.en.Low()
	time.Sleep(100 * time.Microsecond) // commands need > 37us to settle
}

func (g *GPIO) write8BitMode(data byte) {
	g.setPins(data)
	g.pulseEnable()
}

func printBit(word, mask byte) {
	bit := 0
	if mask&word == mask {
		bit = 1
	}
	print(bit)
}

func printNibble(word byte, nl bool) {
	printBit(word, 0x8)
	printBit(word, 0x4)
	printBit(word, 0x2)
	printBit(word, 0x1)
	if nl {
		println()
	} else {
		print(" ")
	}
}

func debugWrite(write func(byte), onlyLow bool) func(byte) {
	return func(data byte) {
		highNibble := data >> 4
		if !onlyLow {
			printNibble(highNibble, false)
		}

		printNibble(data, true)

		write(data)
	}
}

func (g *GPIO) write4BitNibble(data byte) {
	g.setPins(data)
	g.pulseEnable()
}

func (g *GPIO) write4BitMode(data byte) {
	g.write4BitNibble(data >> 4)
	g.write4BitNibble(data)
}

// Read reads len(data) bytes from display RAM to data starting from RAM address counter position
// Ram address can be changed by writing address in command mode
func (g *GPIO) Read(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, errors.New("length greater than 0 is required")
	}
	g.rw.High()
	g.reconfigureGPIOMode(machine.PinInput)
	for i := 0; i < len(data); i++ {
		data[i] = g.read()
		n++
	}
	g.reconfigureGPIOMode(machine.PinInput)
	return n, nil
}

func (g *GPIO) read4BitMode() byte {
	g.en.High()
	data := (g.pins() << 4 & 0xF0)
	g.en.Low()
	g.en.High()
	data |= (g.pins() & 0x0F)
	g.en.Low()
	return data
}

func (g *GPIO) read8BitMode() byte {
	g.en.High()
	data := g.pins()
	g.en.Low()
	return data
}

func (g *GPIO) reconfigureGPIOMode(mode machine.PinMode) {
	for i := 0; i < len(g.dataPins); i++ {
		g.dataPins[i].Configure(machine.PinConfig{Mode: mode})
	}
}

// setPins sets high or low state on all data pins depending on data
func (g *GPIO) setPins(data byte) {
	mask := byte(1)
	for i := 0; i < len(g.dataPins); i++ {
		if (data & mask) != 0 {
			g.dataPins[i].High()
		} else {
			g.dataPins[i].Low()
		}
		mask = mask << 1
	}
}

// pins returns current state of data pins. MSB is D7
func (g *GPIO) pins() byte {
	bits := byte(0)
	for i := uint8(0); i < uint8(len(g.dataPins)); i++ {
		if g.dataPins[i].Get() {
			bits |= (1 << i)
		}
	}
	return bits
}
