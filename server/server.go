package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// Configuration
var recent = 5 // number of 'recent' blocks

// Block struct
type Block struct {
	Buffer []int32 `json:"buffer"`
	I      int     `json:"i"`
}

// Status struct
type Status struct {
	I       int  `json:"i"`
	Success bool `json:"success"`
}

const sampleRate = 44100
const bufferTime = 1
const bufferSize = sampleRate * bufferTime

var buffer []int32 = make([]int32, bufferSize)

var blocks []Block

func main() {
	// Init router
	r := mux.NewRouter()

	// Route handlers
	r.HandleFunc("/audio/latest", getLatestBlockNum).Methods("GET")
	r.HandleFunc("/audio/{i}", getLatestBlock).Methods("GET")
	r.HandleFunc("/audio", setBlock).Methods("POST")

	const songsDir = "./server/streams"
	r.PathPrefix("/").Handler(http.StripPrefix("/", addHeaders(http.FileServer(http.Dir(songsDir)))))

	// Run server
	// fmt.Println("Server started.")
	const port = 8080
	fmt.Printf("Started server on port %v.\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), r))
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

// addHeaders will act as middleware to give us CORS support
func addHeaders(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpegurl")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
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
	status := &Status{I: i, Success: true}
	json.NewEncoder(w).Encode(status)
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
	// fmt.Println(buffer)
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
	body, _ := ioutil.ReadAll(r.Body)
	responseReader := bytes.NewReader(body)

	blockNum := len(blocks)

	path := "server/streams/stream_id"
	newpath := filepath.Join(".", path)
	os.MkdirAll(newpath, os.ModePerm)

	fileName := path + fmt.Sprintf("/stream_%d", blockNum)
	if !strings.HasSuffix(fileName, ".aiff") {
		fileName += ".aiff"
	}
	f, err := os.Create(fileName)
	chk(err)

	// form chunk
	_, err = f.WriteString("FORM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //total bytes
	_, err = f.WriteString("AIFF")
	chk(err)

	// common chunk
	_, err = f.WriteString("COMM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(18)))                  //size
	chk(binary.Write(f, binary.BigEndian, int16(1)))                   //channels
	chk(binary.Write(f, binary.BigEndian, int32(0)))                   //number of samples
	chk(binary.Write(f, binary.BigEndian, int16(32)))                  //bits per sample
	_, err = f.Write([]byte{0x40, 0x0e, 0xac, 0x44, 0, 0, 0, 0, 0, 0}) //80-bit sample rate 44100
	chk(err)

	// sound chunk
	_, err = f.WriteString("SSND")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //size
	chk(binary.Write(f, binary.BigEndian, int32(0))) //offset
	chk(binary.Write(f, binary.BigEndian, int32(0))) //block
	nSamples := 0

	in := make([]int32, bufferSize)
	binary.Read(responseReader, binary.BigEndian, &in)
	chk(binary.Write(f, binary.BigEndian, in))
	// fmt.Println(in)
	nSamples += len(in)

	// fill in missing sizes
	totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
	_, err = f.Seek(4, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(totalBytes)))
	_, err = f.Seek(22, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(nSamples)))
	_, err = f.Seek(42, 0)
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
	chk(f.Close())

	// input := path + "/input.txt"
	// file, err := os.OpenFile(input, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	// if err != nil {
	// 	log.Println(err)
	// }
	// if _, err := file.WriteString(fmt.Sprintf("file 'stream_%d.aiff'\n", blockNum)); err != nil {
	// 	log.Fatal(err)
	// }
	// //Print the contents of the file
	// data, err := ioutil.ReadFile(input)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(string(data))
	// chk(file.Close())

	// do ffmpeg
	// -hls_playlist_type event -hls_flags append_list+omit_endlist hls/songs/recording/recording.m3u8
	// ffmpeg := "ffmpeg -f concat -i server/streams/stream_id/input.txt -c copy -c:a libmp3lame -b:a 128k -map 0:0 -f hls -hls_time 4 -hls_playlist_type event server/streams/stream_id/output.m3u8"
	ffmpeg := fmt.Sprintf("ffmpeg -i server/streams/stream_id/stream_%d.aiff -filter:v 'setpts=1.0' -c:a libmp3lame -b:a 128k -map 0:0 -f hls -hls_time 1 -hls_playlist_type event -hls_flags append_list+omit_endlist server/streams/stream_id/stream.m3u8", blockNum)
	ffmpegArgs := strings.Split(ffmpeg, " ")

	cmd := exec.Command(ffmpegArgs[0], ffmpegArgs[1:]...)
	err = cmd.Run()

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	block := Block{Buffer: in, I: blockNum}
	blocks = append(blocks, block)
	fmt.Printf("Set block: %d\n", block.I)
	status := Status{I: block.I, Success: true}
	json.NewEncoder(w).Encode(status)
}
