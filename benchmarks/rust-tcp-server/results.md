# benchmarked against a multi-threaded rust server
go.net client, 8 connections, one per goroutine:
```
min/avg/max/stddev = 12/31/2629/19us
min/avg/max/stddev = 13/31/2661/20us
min/avg/max/stddev = 12/31/2650/20us
min/avg/max/stddev = 13/31/2658/19us
min/avg/max/stddev = 13/31/2669/19us
min/avg/max/stddev = 12/31/2581/19us
min/avg/max/stddev = 13/31/2624/20us
min/avg/max/stddev = 12/32/2637/18us
min/avg/max/stddev = 12/30/113/10us
min/avg/max/stddev = 12/30/122/10us
min/avg/max/stddev = 12/30/104/11us
min/avg/max/stddev = 12/30/118/11us
min/avg/max/stddev = 12/30/519/11us
min/avg/max/stddev = 12/30/99/10us
min/avg/max/stddev = 13/30/323/10us
min/avg/max/stddev = 13/30/299/10us
```

sonic client, 8 connections, single thread/goroutine
```
min/avg/max/stddev = 13/70/569/19us
min/avg/max/stddev = 10/70/567/19us
min/avg/max/stddev = 14/70/567/20us
min/avg/max/stddev = 13/71/562/17us
min/avg/max/stddev = 24/71/568/17us
min/avg/max/stddev = 12/71/569/17us
min/avg/max/stddev = 15/71/568/17us
min/avg/max/stddev = 10/71/569/18us
```

with all goroutines sharing the same buffer under a lock:
```
min/avg/max/stddev = 11/286/5382/684us
min/avg/max/stddev = 11/274/6497/670us
min/avg/max/stddev = 11/279/5249/676us
min/avg/max/stddev = 11/277/5648/676us
min/avg/max/stddev = 11/283/6248/686us
min/avg/max/stddev = 11/296/5176/700us
min/avg/max/stddev = 11/286/5395/689us
```
```go
lck.Lock()
n, err := conn.Read(b)
lck.Unlock()
if err != nil {
        panic(err)
}
if n != 128 {
        panic("not 128")
}
lck.Lock()
n, err = conn.Write(b)
lck.Unlock()
if err != nil {
        panic(err)
}
```
