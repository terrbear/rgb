package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/kbinani/screenshot"
	"golang.org/x/net/ipv4"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

const ssdpAddress = "239.255.255.250:1982"

func getLightAddress() string {
	udpAddr, err := net.ResolveUDPAddr("udp4", ssdpAddress)
	must(err)

	conn, err := net.ListenUDP("udp4", nil)
	must(err)
	defer conn.Close()

	pConn := ipv4.NewPacketConn(conn)
	pConn.JoinGroup(nil, udpAddr)

	request, err := http.NewRequest("M-SEARCH", "*", nil)
	must(err)
	request.Host = ssdpAddress
	request.Header["MAN"] = []string{`"ssdp:discover"`}
	request.Header["ST"] = []string{"wifi_bulb"}
	raw, err := httputil.DumpRequest(request, true)
	must(err)
	//fmt.Println(string(raw))

	_, err = pConn.WriteTo(raw, nil, udpAddr)
	must(err)

	buffer := make([]byte, 1024)
	n, _, _, err := pConn.ReadFrom(buffer)
	must(err)
	//fmt.Println(addr)
	//fmt.Println(string(buffer[:n]))

	lines := strings.Split(string(buffer[:n]), "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "location:") {
			return strings.ReplaceAll(strings.TrimSpace(line)[len("location: "):], "yeelight://", "")
		}
		//fmt.Println("line: ", line)
	}

	return ""
}

func xmain() {
	l := NewLight()

	l.TurnOn()

	time.Sleep(2 * time.Second)
	fmt.Println("going red")
	l.SetColor(0xFF, 0x00, 0x00)

	time.Sleep(2 * time.Second)
	fmt.Println("going green")
	l.SetColor(0x00, 0xFF, 0x00)

	time.Sleep(2 * time.Second)
	fmt.Println("going blue")
	l.SetColor(0x00, 0x00, 0xFF)

	time.Sleep(2 * time.Second)
	fmt.Println("bye!")
}

type message struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params []any  `json:"params"`
}

type Light struct {
	location string
	msgs     chan message
}

type YeeLightMethod string

const (
	m_SetPower  YeeLightMethod = "set_power"
	m_SetRGB    YeeLightMethod = "set_rgb"
	m_SetBright YeeLightMethod = "set_bright"
)

func NewMessage(method YeeLightMethod, params ...any) message {
	return message{
		ID:     rand.Int(),
		Method: string(method),
		Params: params,
	}
}

func NewLight() *Light {
	l := &Light{
		location: getLightAddress(),
		msgs:     make(chan message),
	}
	go l.run()
	return l
}

func (l *Light) run() {
	conn, err := net.Dial("tcp", l.location)
	must(err)

	go func() {
		for {
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			must(err)

			fmt.Println("reply: ", string(buffer[:n]))
		}
	}()

	encoder := json.NewEncoder(conn)

	for msg := range l.msgs {
		must(encoder.Encode(msg))
		_, err := conn.Write([]byte("\r\n"))
		must(err)
	}

}

func (l *Light) TurnOn() {
	l.msgs <- NewMessage(m_SetPower, "on", "smooth", 500)
}

func (l *Light) SetColor(red, green, blue int) {
	//fmt.Println("setting color to: ", red, green, blue)
	rgb := red<<16 | green<<8 | blue
	l.msgs <- NewMessage(m_SetRGB, rgb, "smooth", 500)
}

func (l *Light) SetBrightness(brightness int) {
	//fmt.Println("setting brightness to: ", brightness)
	l.msgs <- NewMessage(m_SetBright, brightness, "smooth", 500)
}

func main() {
	fmt.Println("running 2.0")

	l := NewLight()
	l.TurnOn()

	t := time.NewTicker(100 * time.Millisecond)

	samples := make(map[int]uint8)
	errcount := 0

	for {
		<-t.C

		bounds := screenshot.GetDisplayBounds(0)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			if errcount < 10 {
				errcount++
				continue
			}
			panic(err)
		}

		pixels := 1.0

		rgba := []float64{0, 0, 0, 0}

		if len(samples) == 0 {
			for i := 0; i < len(img.Pix); i += len(img.Pix) / 20 {
				samples[i] = img.Pix[i]
			}
		} else {
			changed := false
			//fmt.Println("checking samples")
			for i, v := range samples {
				if img.Pix[i] != v {
					changed = true
					samples[i] = img.Pix[i]
				}
			}
			if !changed {
				//fmt.Println("nothing to do")
				continue
			}
		}

		for i := 0; i < len(img.Pix); i += 4 {
			r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
			pixels++
			rgba[0] = rgba[0]*(1-(1/pixels)) + float64(r)*(1/pixels)*(float64(a)/0xFF)
			rgba[1] = rgba[1]*(1-(1/pixels)) + float64(g)*(1/pixels)*(float64(a)/0xFF)
			rgba[2] = rgba[2]*(1-(1/pixels)) + float64(b)*(1/pixels)*(float64(a)/0xFF)
			rgba[3] = rgba[3]*(1-(1/pixels)) + float64(a)*(1/pixels)
		}

		red := 0xFF & uint32(math.Round(rgba[0]))
		green := 0xFF & uint32(math.Round(rgba[1]))
		blue := 0xFF & uint32(math.Round(rgba[2]))

		l.SetColor(int(red), int(green), int(blue))
		brightness := int(float64(red+green+blue) / (3 * 0xFF) * 100)
		//fmt.Println("brightness avg: ", brightness)
		l.SetBrightness(brightness)
		errcount = 0
	}
}
