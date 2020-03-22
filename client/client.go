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

type Latest struct {
	Success bool `json:"success"`
	I       int  `json:"i"`
}

// type Chunk struct {
// 	Blocks []Block `json:"blocks"`
// }

// type Block struct {
// 	Buffer []float32 `json:"buffer"`
// 	I      int       `json:"i"`
// }

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
	var latest Latest
	err = json.Unmarshal(body, &latest)
	chk(err)
	i = latest.I
	fmt.Printf("Latest block: %d\n", i)

	buffer := make([]float32, bufferSize)

	// updateChunk(0)
	stream, err := portaudio.OpenDefaultStream(0, 1, sampleRate, bufferSize, func(out []float32) {
		// resp, err := http.Get("http://localhost:8080/audio")
		resp, err := http.Get(fmt.Sprintf("http://localhost:8080/audio/%d", i))
		chk(err)
		body, _ := ioutil.ReadAll(resp.Body)
		responseReader := bytes.NewReader(body)
		binary.Read(responseReader, binary.BigEndian, &buffer)
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
	for {
		select {
		case <-sig:
			return
		default:
		}
	}
	chk(stream.Stop())
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

// func updateChunk(i int) {
// 	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/audio/%d", i))
// 	chk(err)
// 	body, _ := ioutil.ReadAll(resp.Body)
// 	err = json.Unmarshal(body, &chunk)
// 	// fmt.Println("Updated Chunk")
// }
