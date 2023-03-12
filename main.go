package main

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbinani/screenshot"
)

const pongWait = 10 * time.Second

type Payload struct {
	Endpoint string `json:"endpoint"`
	Effect   string `json:"effect"`
	Token    int64  `json:"token"`
	Param    struct {
		Color uint32 `json:"color"`
	} `json:"param"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fmt.Println("running 1.2")

	t := time.NewTicker(50 * time.Millisecond)

	ws, _, err := websocket.DefaultDialer.Dial("wss://chromasdk.io:13339/razer/chromasdk", nil)
	if err != nil {
		panic(err)
	}

	if err := ws.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(pongWait)) })

	var app struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Author      struct {
			Name    string `json:"name"`
			Contact string `json:"contact"`
		} `json:"author"`
		DeviceSupported []string `json:"device_supported"`
		Category        string   `json:"category"`
	}

	app.Title = "terry rgb"
	app.Description = "blah"
	app.Author.Name = "terry"
	app.Author.Contact = "terrbear.io"
	app.DeviceSupported = []string{"keyboard", "chromalink", "mouse"}
	app.Category = "application"

	pinger := time.NewTicker(pongWait)

	out, err := json.Marshal(app)
	must(err)

	must(ws.WriteMessage(websocket.TextMessage, out))

	msg := make(chan uint32)

	p := Payload{
		Endpoint: "chromalink",
		Effect:   "CHROMA_STATIC",
		Token:    time.Now().UnixMilli(),
	}

	go func() {
		for {
			select {
			case <-pinger.C:
				ws.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
				must(ws.WriteMessage(websocket.PingMessage, []byte("ping")))
			case m := <-msg:
				if m == p.Param.Color {
					continue
				}

				p.Param.Color = m

				must(ws.SetWriteDeadline(time.Now().Add(500 * time.Millisecond)))

				out, err := json.Marshal(p)
				must(err)

				must(ws.WriteMessage(websocket.TextMessage, out))
			}
		}
	}()

	/*
		for i := uint32(0xFF); i < 0xFF000000; i <<= 1 {
			fmt.Printf("color: %b\n", i)
			msg <- i
			time.Sleep(3 * time.Second)
		}

		fmt.Printf("going blue %d (%b)\n", rgb2bgr(0x0c), rgb2bgr(0x0c))
		msg <- rgb2bgr(0x0c)
		msg <- 0xFF000000
		time.Sleep(3 * time.Second)

		fmt.Printf("going red %d (%b)\n", rgb2bgr(0xc0), rgb2bgr(0xc0))
		msg <- rgb2bgr(0xc0)
		time.Sleep(3 * time.Second)

		fmt.Printf("going green %d (%b)\n", rgb2bgr(0x30), rgb2bgr(0x30))
		msg <- rgb2bgr(0x30)
		time.Sleep(3 * time.Second)
	*/

	errcount := 0

	samples := make(map[int]uint8)

	for {
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
			for i, v := range samples {
				if img.Pix[i] != v {
					changed = true
					samples[i] = img.Pix[i]
				}
			}
			if !changed {
				continue
			}
		}

		for i, p := range img.Pix {
			component := rgba[i%4]
			pixels++
			rgba[i%4] = component*(1-(1/pixels)) + float64(p)*(1/pixels)
		}

		red := 0xFF & uint32(math.Round(rgba[0]))
		green := 0xFF & uint32(math.Round(rgba[1]))
		blue := 0xFF & uint32(math.Round(rgba[2]))

		if red > green && red > blue {
			red |= 0x80
		} else if green > red && green > blue {
			green |= 0x80
		} else if blue > red && blue > green {
			blue |= 0x80
		}

		bgr := red | green<<8 | blue<<16

		msg <- bgr
		errcount = 0
		<-t.C
	}
}
