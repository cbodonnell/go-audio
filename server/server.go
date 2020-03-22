package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/gordonklaus/portaudio"
)

// Configuration
var recent = 5 // number of 'recent' blocks

// Block struct
type Block struct {
	Buffer []float32 `json:"buffer"`
	I      int       `json:"i"`
}

// Status struct
type Status struct {
	I      int  `json:"i"`
	Status bool `json:"success"`
}

// Latest struct
type Latest struct {
	Success bool `json:"success"`
	I       int  `json:"i"`
}

const sampleRate = 44100
const bufferTime = 1
const bufferSize = sampleRate * bufferTime

var buffer []float32 = make([]float32, bufferSize)

var blocks []Block

func main() {

	portaudio.Initialize()
	defer portaudio.Terminate()

	stream, err := portaudio.OpenDefaultStream(1, 0, sampleRate, len(buffer), func(in []float32) {
		for i := range buffer {
			buffer[i] = in[i]
		}
	})

	chk(err)
	chk(stream.Start())
	fmt.Println("Stream started.")
	defer stream.Close()

	// Init router
	r := mux.NewRouter()

	// Route handlers
	r.HandleFunc("/audio/latest", getLatestBlockNum).Methods("GET")
	r.HandleFunc("/audio/{i}", getLatestBlock).Methods("GET")
	r.HandleFunc("/audio", setBlock).Methods("POST")

	// Run server
	log.Fatal(http.ListenAndServe(":8080", r))
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

func getBlock(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	i, err := strconv.Atoi(params["i"])
	chk(err)
	fmt.Printf("Getting block: %d\n", i)
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}

	buffer = blocks[i].Buffer

	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "audio/wave")
	for true {
		binary.Write(w, binary.BigEndian, &buffer)
		flusher.Flush()
		return
	}
}

func getLatestBlockNum(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	i := len(blocks) - 1
	fmt.Printf("Latest block num: %d\n", i)
	latest := &Latest{Success: true, I: i}
	json.NewEncoder(w).Encode(latest)
}

func getLatestBlock(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	i, err := strconv.Atoi(params["i"])
	chk(err)
	fmt.Printf("Getting block: %d\n", i)
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}

	recent := getRecentBlocks()
	buffer := recent[len(recent)-1].Buffer
	for _, block := range recent {
		if block.I == i {
			buffer = block.Buffer
		}
	}

	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "audio/wave")
	for true {
		binary.Write(w, binary.BigEndian, &buffer)
		flusher.Flush()
		return
	}
}

func getRecentBlocks() []Block {
	n := len(blocks)
	if n < recent {
		return blocks[n-1 : n]
	}
	return blocks[n-recent : n]
}

func setBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var block Block
	err := json.NewDecoder(r.Body).Decode(&block)
	chk(err)
	blocks = append(blocks, block)
	fmt.Printf("Set block: %d\n", block.I)
	status := &Status{I: block.I, Status: true}
	json.NewEncoder(w).Encode(status)
}
