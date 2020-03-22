package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"

	"github.com/gordonklaus/portaudio"
)

// Block struct
type Block struct {
	Buffer []float32 `json:"buffer"`
	I      int       `json:"i"`
}

const sampleRate = 44100
const bufferTime = 1
const bufferSize = sampleRate * bufferTime

func main() {

	portaudio.Initialize()
	defer portaudio.Terminate()
	buffer := make([]float32, bufferSize)
	block := &Block{Buffer: buffer, I: 0}

	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(buffer), func(in []float32) {
		for i := range buffer {
			block.Buffer[i] = in[i]
		}

		b, err := json.Marshal(block)
		chk(err)
		url := "http://localhost:8080/audio"
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
		chk(err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		chk(err)
		defer resp.Body.Close()
		fmt.Printf("Sent block: %d\n", block.I)
		block.I++

		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println("response Body:", string(body))
	})
	chk(err)
	defer stream.Close()

	chk(stream.Start())
	fmt.Println("Broadcasting.  Press Ctrl-C to stop.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	waitForSignal(sig)
	chk(stream.Stop())
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

func waitForSignal(sig chan os.Signal) {
	for {
		select {
		case <-sig:
			return
		default:
		}
	}
}
