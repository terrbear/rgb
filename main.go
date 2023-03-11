package main

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kbinani/screenshot"
)

const pongWait = 1 * time.Second

type Payload struct {
	Endpoint string `json:"endpoint"`
	Effect   string `json:"effect"`
	Param    struct {
		Color uint8 `json:"color"`
	} `json:"param"`
}

func main() {
	n := screenshot.NumActiveDisplays()

	t := time.NewTicker(50 * time.Millisecond)

	ws, _, err := websocket.DefaultDialer.Dial("wss://chromasdk.io:13339/razer/chromasdk", nil)
	if err != nil {
		panic(err)
	}

	if err := ws.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(pongWait)) })

	pinger := time.NewTicker(pongWait)

	msg := make(chan uint8)

	go func() {
		for {
			select {
			case <-pinger.C:
				ws.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
				ws.WriteMessage(websocket.PingMessage, []byte("ping"))
			case m := <-msg:
				p := Payload{
					Endpoint: "chromalink",
					Effect:   "CHROMA_STATIC",
				}
				p.Param.Color = m
				ws.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))

				out, err := json.Marshal(p)
				if err != nil {
					panic(err)
				}
				if err := ws.WriteMessage(websocket.TextMessage, out); err != nil {
					panic(err)
				}
			}
		}
	}()

	for {
		for i := 0; i < n; i++ {
			bounds := screenshot.GetDisplayBounds(i)

			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				panic(err)
			}

			avg := float64(img.Pix[0])
			pixels := 1.0

			for _, p := range img.Pix[1:] {
				pixels++
				avg = avg*(1-(1/pixels)) + float64(p)*(1/pixels)
			}

			flipped := uint8(math.Round(avg))

			fmt.Println("Flipped", flipped)
			msg <- flipped
		}
		<-t.C
	}
}
