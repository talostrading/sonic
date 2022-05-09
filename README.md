*this is currently work-in-progress*

# Sonic
Sonic is a Go library for network and I/O programming that provides developers with a consistent asynchronous model. Sonic supports only Unix based systems. 

### Why?
- writing low latency/variance code in Go is not possible by using a synchronous model based on blocking sockets and goroutines. Why?
  - since each read/write is blocking, each file descriptor must be handled by a goroutine
  - although goroutines are cheap to spawn, they are expensive to run because:
    - the Go scheduler can at anytime swap the thread on which a goroutine is running
    - the goroutine can be paused pre-emptively by the scheduler
  - as such, the scheduler will introduce ms of latency if there are more than 2 goroutines within a process

- single threaded/goroutine programs are easier to write and reason about than their multi-threaded counterparts as there's no need for synchronization

### How?
Check `./examples`

### Credits
- [boost.asio](https://www.boost.org/doc/libs/1_75_0/doc/html/boost_asio.html)
- [boost.beast](https://github.com/boostorg/beast)
- [mio](https://github.com/tokio-rs/mio)

![](https://c.tenor.com/OTDlqAguqpEAAAAi/sonic-running.gif)
