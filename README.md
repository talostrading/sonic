*this is currently work-in-progress - expect breaking changes until v1.0.0*

# Sonic
Sonic is a Go library for network and I/O programming that provides developers with a consistent asynchronous model, with a focus on achieving the lowest possible latency and jitter.

It is an alternative to the `net` package that removes the need of using goroutines to handle
multiple connections and reads/writes in the same process. By doing that, a single goroutine
and thread is used which brings several benefits:
- No need to use synchronization primitives (channels, mutexes etc.) as multiple connections can be handled in the same goroutine.
- It removes the need for the Go scheduler to do any work which could slow down the program.
- It allows latency sensitive programs to run in a hot-loop pinned to a thread on an isolated core in order to achieve low latency and jitter.

Sonic currently supports only Unix based systems.

```go
func main() {
    // Create an IO object which can execute asynchronous operations on the
    // current goroutine. 
    ioc := sonic.MustIO()
    defer ioc.Close()

    // Create 10 connections. Each connection reads a message into it's
    // buffer and then closes.
    for i := 0; i < 10; i++ {
        conn, _ := sonic.Dial(ioc, "tcp", "localhost:8080")
		
        b := make([]byte, 128)
        conn.AsyncRead(b, func(err error, n int) {
            if err != nil {
                fmt.Printf("could not read from %d err=%v\n", i, err)
            } else {
                b = b[:n]
                fmt.Println("got=", string(b))
                conn.Close()
            }
        })
    }

    // Execute all pending reads scheduled in the for-loop, then exit.
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

