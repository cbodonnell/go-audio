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

// Status struct
type Status struct {
	I       int  `json:"i"`
	Success bool `json:"success"`
}

const sampleRate = 44100
const bufferTime = 1
const bufferSize = sampleRate * bufferTime

var i int = 0

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()

	resp, err := http.Get("http://localhost:8080/audio/latest")
	chk(err)
	body, _ := ioutil.ReadAll(resp.Body)
	var status Status
	err = json.Unmarshal(body, &status)
	chk(err)
	i = status.I
	fmt.Printf("Latest block: %d\n", i)

	buffer := make([]int32, bufferSize)
	stream, err := portaudio.OpenDefaultStream(0, 1, sampleRate, len(buffer), func(out []int32) {
		resp, err := http.Get(fmt.Sprintf("http://localhost:8080/audio/%d", i))
		chk(err)
		body, _ := ioutil.ReadAll(resp.Body)
		responseReader := bytes.NewReader(body)
		binary.Read(responseReader, binary.BigEndian, &buffer)
		// fmt.Println(buffer)
		for i := range out {
			out[i] = buffer[i]
		}
		i++
	})
	chk(err)
	chk(stream.Start())
	fmt.Println("Listening.  Press Ctrl-C to stop.")

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
