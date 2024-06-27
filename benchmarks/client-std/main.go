package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/talostrading/sonic/util"
)

var (
	n       = flag.Int("n", 10, "number of connections")
	samples = flag.Int("samples", 1024, "number of samples to record")
	rate    = flag.Int("rate", 0, "rate")
)

func handle(id int, conn net.Conn, samples []time.Duration) {
	defer conn.Close()

	ignore := 0
	index := 0
	b := make([]byte, 1024)
	var (
		period   int64 = math.MaxInt64
		lastTime int64 = 0 // nanos epoch
	)

	for {
		nTotal := 0
		for nTotal != 1024 {
			n, err := conn.Read(b[nTotal:])
			if err != nil {
				panic(err)
			}
			nTotal += n
		}
		if ignore < 100 {
			ignore++
			continue
		}

		now := util.GetMonoTimeNanos()
		if index < len(samples) {
			samples[index] = time.Duration(now-int64(binary.LittleEndian.Uint64(b))) * time.Nanosecond
			index++

			if lastTime > 0 {
				diff := now - lastTime
				if period > diff {
					period = diff
				}
			}
			lastTime = now
		} else {
			fmt.Println("connection has finished", id, "min_period", time.Duration(period)*time.Nanosecond)
			return
		}
	}

}

func main() {
	debug.SetGCPercent(-1)

	if err := util.PinTo(3, 4, 5); err != nil {
		panic(err)
	}

	flag.Parse()

	if *rate == 0 {
		panic("rate not provided")
	}

	fmt.Printf("%d connections, %d samples per connection, rate=%dHz period=%s\n", *n, *samples, *rate, time.Second/time.Duration(*rate))

	var (
		wg         sync.WaitGroup
		allSamples [][]time.Duration
	)

	for i := 0; i < *n; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		if err := conn.(*net.TCPConn).SetNoDelay(true); err != nil {
			panic(err)
		}
		connSamples := make([]time.Duration, *samples, *samples)
		allSamples = append(allSamples, connSamples)
		wg.Add(1)
		id := i
		go func() {
			defer wg.Done()
			handle(id, conn, connSamples)
		}()
	}

	wg.Wait()

	fmt.Println("all connections have finished")
	filename := fmt.Sprintf("std_%dHz_%d.txt", *rate, *n)
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	for _, samples := range allSamples {
		for _, sample := range samples {
			file.WriteString(fmt.Sprintf("%d\n", sample.Microseconds()))
		}
	}
	fmt.Println("wrote", filename, ", bye :)")
}
