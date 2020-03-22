package main

import (
	"bytes"
	"encoding/binary"
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

// Status struct
type Status struct {
	I       int  `json:"i"`
	Success bool `json:"success"`
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

		buf := new(bytes.Buffer)
		for _, v := range block.Buffer {
			err := binary.Write(buf, binary.BigEndian, v)
			chk(err)
		}

		url := "http://localhost:8080/audio"
		req, err := http.NewRequest("POST", url, buf)
		chk(err)
		req.Header.Set("Connection", "Keep-Alive")
		req.Header.Set("Access-Control-Allow-Origin", "*")
		req.Header.Set("X-Content-Type-Options", "nosniff")
		req.Header.Set("Transfer-Encoding", "chunked")
		req.Header.Set("Content-Type", "audio/wave")

		client := &http.Client{}
		resp, err := client.Do(req)
		chk(err)
		defer resp.Body.Close()

		fmt.Println("response Status:", resp.Status)
		fmt.Println("response Headers:", resp.Header)

		body, err := ioutil.ReadAll(resp.Body)
		var status Status
		err = json.Unmarshal(body, &status)
		chk(err)
		fmt.Printf("Sent block: %d\n", status.I)
		block.I = status.I
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
