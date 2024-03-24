*work-in-progress - expect breaking changes until v1.0.0*

# Sonic

Sonic is a Go library for network and I/O programming that provides developers with a consistent asynchronous model,
with a focus on achieving the lowest possible latency and jitter in Go. Sonic aims to make it easy to write network
protocols (websocket, http2, custom exchange binary) on a series of bytestreams and then make use of those bytestreams
through multiple connections running in a **single-thread and goroutine**.

Sonic is an alternative to the `net` package. It removes the need to use multiple goroutines to handle
multiple connections and reads/writes in the same process. By doing that, a single goroutine
and thread is used to read/write from multiple connections which brings several benefits:

- No need to use synchronization primitives (channels, mutexes, etc.) as multiple connections can be handled in the same
  goroutine.
- It removes the need for the Go scheduler to do any work which could slow down the program.
- It allows latency-sensitive programs to run in a hot-loop pinned to a thread on an isolated core in order to achieve
  low latency and jitter.

Sonic currently supports only Unix-based systems (BSD, macOS, Linux).

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

See `examples/`. A good starting point is `examples/timer`. All examples can be built by calling `make` in the root path
of sonic. The builds will be put in `bin/`.

### UDP Multicast

`sonic` offers a full-featured `UDP Multicast` peer for both `IPv4` and `IPv6`. See `multicast/peer.go`. This peer can
read and write data to a multicast group, join a group with source-IP and network interface filtering, and control its
group membership by blocking/unblocking source-IPs at runtime.

Moreover, this peer, unlike the `websocket` client, does not allocate and copy any data in any of its functions.
Additionally, the peer gives the programmer the option to change its read buffer after scheduling a read on it i.e.
```go
var (
    b1, b2 []byte
)
peer.AsyncRead(b1, func(...) { ... }) // schedule an asynchronous read in b1
// ... some other code here
peer.SetAsyncReadBuffer(b2) // make the previously scheduled asynchronous read use b2 instead of b1
```

This is very useful when multiple `UDP` peers share the same read buffer. For example:

```go
b := make([]byte, 1024 * 1024)

// We expect packets to be less than 256 bytes. When either peer reads, it calls the updateAndProcessBuffer function.
peer1.AsyncRead(b[:256], func(...) { updateAndProcessBuffer() })
peer2.AsyncRead(b[:256], func(...) { updateAndProcessBuffer() })

func updateAndProcessBuffer() {
    // One of the peers read something. We instruct the peers to read into the next 256 byte chunk of b such that we can
    // process the previous 256 bytes.
    peer1.AsyncRead(b[256:512], func(...) { updateAndProcessBuffer() })
    peer2.AsyncRead(b[256:512], func(...) { updateAndProcessBuffer() })

    go process(b[:256])
}
```

### Zero-copy FIFO buffers

We provide two types of FIFO buffers with zero-copy semantics. Regardless of the type, a FIFO buffer is essential when
writing protocol encoders/decoders over UDP or TCP with Linux's socket API to minimize syscalls. For example,
say we have a simple protocol where each message has a fixed-size header and a variable-sized payload - the length of
the payload is in the header. Say we read data through TCP. We then have two options:
```go
// buffer in which we read; assume header size is 1 byte.

b := make([]byte, 1024)

// option 1: read the header first and then the payload from the network
conn.Read(b[:1]) // read the header
payloadSize := int(b[0])
payload := b[1:payloadSize]
conn.Read(payload) // read the payload
// do something with the payload

// option 2: read as much as you can from the network and then parse the bytes
conn.Read(b)
i := 0
while i < len(b) {
    payloadSize := int(b[i:i+1])
    if i + 1 + payloadSize <= len(b) {
        payload := b[i+1:i+1+payloadSize]
        process(payload)
    }
    i += 1 + payloadSize
}

```

`option 1` is not efficient as `n` messages need `n * 2` syscalls. `option 2` is efficient as the number if syscalls is
minimized - in the limit, we need just 1 syscall to read `n` messages. `option 2` however is missing something:

- what if the last read message was incomplete i.e. we read the header with its size, say `255`, but only had space to
  read `100` of those bytes into `b` as we're near the end of `b`.
- to read the rest of the `255 - 100 = 155` bytes of the payload, we need to move the read `100` bytes to the beginning
  of `b`, overwriting the already processed payloads.
- in other words, we need FIFO semantics over `b`.

The naive way of offering FIFO semantics over b would be to simply copy the `100` bytes to the beginning of the slice.
But that's a `memcpy` that will take a lot of time if the message is relatively big, say over 1KB. That's not
acceptable even though that's how we do things for websocket (see `byte_buffer.go` and `codec/websocket/codec.go`).
In those cases we offer two types of FIFO semantics over a byte slice, both offering the same API:

- `Claim(n) []byte` - claim at most n bytes from the underlying `[]byte` slice. Callers can now read into the returned
  slice.
- `Commit(n) int` - commit at most n previously claimed bytes i.e. queue at most `n` bytes
- `Consume(n) int` - consume at most n previously committed/queued bytes

#### Mirrored Buffer

This is a zero-copy FIFO buffer that works for both TCP and UDP protocols. It offers contiguous byte slices in a FIFO
manner without care for wrapping. See `bytes/mirrored_buffer.go`. The only limitations are that the buffer size must be
a multiple of the system's page size and the system must expose a shared memory filesystem like `/dev/shm`. In short, the
mirrored buffer provides zero-copy FIFO semantics over a byte slice in the following way:

- it creates the underlying byte slice of size `n` (where `n` is a multiple of page size) and **maps it twice,
  contiguously, in the process' virtual address space** with `mmap`.
- there are `n` physical bytes backing up the underlying byte slice and `n * 2` virtual bytes
- the buffer is mirrored in a sense that reading/writing to the sequence `b[n], b[n+1], ..., b[2*n-1]` is permitted and
  in fact, touches the bytes at `b[0], b[1], ..., b[n-1]`

#### Bip Buffer

This is a zero-copy FIFO buffer meant solely for writing packet-based (UDP) protocols. Refer to the
creator's [post](https://www.codeproject.com/Articles/3479/The-Bip-Buffer-The-Circular-Buffer-with-a-Twist) for an
explanation of how it works.

#### What is next

The two buffers above are not yet standardized across sonic. TCP codecs, including `websocket`, still use the `memcpy`
based byte buffer abstraction `byte_buffer.go` which is not that performant for large messages. The plan is to port
websocket to use the mirrored buffer by `v1.0.0`.

The Bip Buffer is actively used in Talos UDP-based gateways.

## Peculiarities

### Async preemption

If, for some reason, you have a single goroutine that ends up waiting for more than 10ms for something to happen, sonic
will crash on Linux due to `epoll_wait` being interrupted by the signal SIGURG. This happens because, by default, the Go
runtime non-cooperatively preempts goroutines that are idle for more than 10ms. To turn off this behavior,
set `GODEBUG=asyncpreemptoff=1` before running your binary.

This issue has been addressed
in [this](https://github.com/talostrading/sonic/commit/d59145deb86647460abd9e85eddbdb03f50e2b01) commit.

### Credits

- [boost.asio](https://www.boost.org/doc/libs/1_75_0/doc/html/boost_asio.html) - the main inspiration for the sonic API
- [boost.beast](https://github.com/boostorg/beast)
- [mio](https://github.com/tokio-rs/mio)
- [tungstenite-rs](https://github.com/snapview/tungstenite-rs)

<p align="center">
  <img src="https://c.tenor.com/OTDlqAguqpEAAAAi/sonic-running.gif" />
</p>

