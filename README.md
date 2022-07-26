*this is currently work-in-progress - expect breaking changes until v1.0.0*

# Sonic
Sonic is a Go library for network and I/O programming that provides developers with a consistent asynchronous model. Sonic currently supports only Unix based systems.

```go
func main() {
    ioc := sonic.MustIO()
    defer ioc.Close()

    for i := 0; i < n; i++ {
        conn, _ := sonic.Dial(ioc, "tcp", "localhost:8080")
		
        buf := make([]byte, 128)
        conn.AsyncRead(buf, func(err error, n int) {
          buf = buf[:n]
          fmt.Println("got=", string(buf))
          conn.Close()
        })
    }

    ioc.RunPending()
}
```

## Getting Started
See `examples/`. A good starting point is `examples/timer`. All examples can be built by calling `make` in the root path of sonic. The builds will be put in `bin/`.

For more information, see `docs/`.

Using `sonic` in your own package might require `export GOPRIVATE=github.com/talostrading/sonic`.

## Peculiarities
### Async preemption
If, for some reason, you have a single goroutine which ends up waiting for more than 10ms for something to happen, sonic will crash on Linux due to epoll_wait being interrupted by the signal SIGURG. This happens because, by default, the Go runtime non-cooperatively preempts goroutines which are idle for more than 10ms. To turn off this behaviour, set `GODEBUG=asyncpreemptoff=1` before running your binary.

This issue has been addressed in [this](https://github.com/talostrading/sonic/commit/d59145deb86647460abd9e85eddbdb03f50e2b01) commit.

### Credits
- [boost.asio](https://www.boost.org/doc/libs/1_75_0/doc/html/boost_asio.html)
- [boost.beast](https://github.com/boostorg/beast)
- [mio](https://github.com/tokio-rs/mio)
- [tungstenite-rs](https://github.com/snapview/tungstenite-rs)

<p align="center">
  <img src="https://c.tenor.com/OTDlqAguqpEAAAAi/sonic-running.gif" />
</p>
