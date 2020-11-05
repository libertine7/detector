package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gonum/stat"
	"github.com/gorilla/websocket"
)

func myavg(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func worker(datachanel chan []byte, metricchannel chan [8000]float64) {
	var metricCount int64
	var errorCount int64
	start := time.Now()

	// metricChannels := []uint16{455}
	metricChannels := GetChannels()

	timeMetrics := make(map[int][]float64)

	var channelSize int64 = 1024
	// tempMetrics := make([]float64, channelSize)

	for _, v := range metricChannels {
		timeMetrics[v] = make([]float64, channelSize)
	}

channelLoop:
	for {
		mydata, more := <-datachanel
		if !more {
			return
		}

		// check for header
		if !bytes.HasPrefix(mydata, []byte{0xff, 0xff}) {
			continue channelLoop
		}

		metricCount++

		metriclen := len(mydata) / 4
		//	metric := make([]uint16, metriclen)
		var metric [8000]float64

		// extract from big endian
		for i := 1; i < metriclen; i++ {
			index := uint16(uint16(mydata[4*i+1]) | uint16(mydata[4*i])<<8)
			if int(index) != i-1 {
				errorCount++
				continue channelLoop
			}
			metric[index] = float64(uint16(uint16(mydata[4*i+3]) | uint16(mydata[4*i+2])<<8))
		}

		for _, v := range metricChannels {
			copy(timeMetrics[v], timeMetrics[v][1:channelSize])
			timeMetrics[v][channelSize-1] = metric[v]
		}

		//temp metric
		// copy(tempMetrics, tempMetrics[1:channelSize])
		// tempMetrics[channelSize-1] = math.Sqrt(metric[681]*metric[681] + metric[801]*metric[801])

		// for first metrics
		if metricCount < 2000 {
			continue channelLoop
		}

		// send data to ws
		if metricCount%256 == 0 {
			metricchannel <- metric
		}

		// print current data
		if metricCount%256 == 0 {
			elapsed := time.Since(start)
			fmt.Println("---------------------------------")
			fmt.Println(metricCount, errorCount, int64(float64(metricCount)/elapsed.Seconds()))

			for _, v := range metricChannels {
				// channelHealth := stat.StdDev(metric[v-50:v+50], nil)
				timeStdDev := stat.StdDev(timeMetrics[v], nil)
				// fmt.Println("----")
				// fmt.Println("Channel health StdDev", v, channelHealth)
				fmt.Println("Time StdDev channel", v, timeStdDev)
			}
			// tempStdDev := stat.StdDev(tempMetrics, nil)
			// fmt.Println("Temp StdDev ", tempStdDev)
		}

		if metricCount%256 == 0 {
			for _, v := range metricChannels {
				channelHealth := stat.StdDev(metric[v-50:v+50], nil)
				timeStdDev := stat.StdDev(timeMetrics[v], nil)
				if channelHealth < 200 {
					// fmt.Println("Sabotage channelHealth", v, channelHealth)
					SetChannelState(v, "sabotage")
				} else if timeStdDev > 60 {
					SetChannelState(v, "alarm")
					// fmt.Println("Alarm", v, timeStdDev)
				} else {
					SetChannelState(v, "OK")
				}
			}
		}
	}
}

func serveDriver(datachanel chan []byte) {
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", ":8080")

	if err != nil {
		log.Fatal(err)
	}
	defer pc.Close()

	fmt.Println("start ")
	fmt.Println("LocalAddr ", pc.LocalAddr())

	var bb bytes.Buffer

	for {
		buf := make([]byte, 1024)
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		if bytes.HasPrefix(buf[:n], []byte{0xff, 0xff}) {
			datachanel <- bb.Bytes()
			bb.Reset()
		}

		bb.Write(buf[:n])
	}
}

var clients = make(map[*websocket.Conn]bool)

var upgrader = websocket.Upgrader{
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	// ws.SetCompressionLevel(9)
	if err != nil {
		log.Fatal(err)
	}

	// register client
	clients[ws] = true
}

func echo(broadcast chan [8000]float64) {
	for {
		val := <-broadcast

		for client := range clients {
			err := client.WriteJSON(val)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func mmmain() {

	datachanel := make(chan []byte, 100)
	metricchannel := make(chan [8000]float64, 100)
	go worker(datachanel, metricchannel)
	go serveDriver(datachanel)
	go echo(metricchannel)

	fs := http.FileServer(http.Dir("static"))

	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)
	http.ListenAndServe(":8118", nil)
}
