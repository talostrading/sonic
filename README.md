*this is currently work-in-progress*

# Sonic
Sonic is a Go library for network and I/O programming that provides developers with a consistent asynchronous model. Sonic supports only Unix based systems. 

### Why?
#### Decoupling threads from concurrency
- a library like `sonic` is typically used to achieve higher efficiency of the application by eliminating the wait for a particular function to complete while the application can perform other tasks. Any application that is heavily IO-bound should use `sonic`, as the applications won't need to spawn additional threads in order to increase concurrency.

#### Performance and scalability
- writing low latency/variance code in Go is not possible by using a synchronous model based on blocking sockets and goroutines. Why?
  - since each read/write is blocking, each file descriptor must be handled by a goroutine in order to do other things while we are blocked on IO
  - although goroutines are cheap to spawn, they are expensive to run because due to increased context-switching and data movement among CPUs:
    - the Go scheduler can at anytime swap the thread on which a goroutine is running
    - the goroutine can be paused pre-emptively by the scheduler
  - as such, the scheduler will introduce ms of latency if there are more than 2 goroutines within a process
  - with asynchronous operations it is possible to avoid the cost of context switching by minimizing the number of OS threads - typically a limited resource - and only activating the logical threads of control that have events to process.

#### Simplified synchronization
- single threaded/goroutine programs are easier to write and reason about than their multi-threaded counterparts as there's no need for synchronization

#### Function composition
- function composition refers to the implementation of functions to provide a higher-level operation, such as sending a message in a particular format. Each function is implemented in terms of multiple calls to lower-level read or write operations.

- for example, consider a protocol where each message consists of a fixed-length header followed by a variable length body, where the length of the body is specified in the header. A hypothetical read_message operation could be implemented using two lower-level reads, the first to receive the header and, once the length is known, the second to receive the body.

- to compose functions in an asynchronous model, asynchronous operations can be chained together. That is, a completion handler for one operation can initiate the next. Starting the first call in the chain can be encapsulated so that the caller need not be aware that the higher-level operation is implemented as a chain of asynchronous operations.

- the ability to compose new operations in this way simplifies the development of higher levels of abstraction above a networking library, such as functions to support a specific protocol.

### How?
Check `./examples`.

### Design
#### Executor
- the core data structure is `IO`: it executes handlers asynchronously for you in the goroutine it was spawned. `IO` guarantees that callback handlers will only be called from goroutines that are currently calling one of the `Run*` functions.

#### Reads/Writes
- all sockets are non-blocking. That means that calling `conn.Read([]byte)` for example will not block the current goroutine if there is nothing to read from the socket. Instead, `ErrWouldBlock` is returned.

- if you want to read something asynchronously, then you can call `conn.AsyncRead([]byte, handler)`. The `handler` is a function invoked by `IO` when the read from the underlying socket is complete.
  - under the hood, we first try to read directly from the socket, invoking `conn.Read`. If that returns `ErrWouldBlock`, then we schedule a read operation using `epoll` on `linux` and `kqueue` on `bsd`.

### Credits
- [boost.asio](https://www.boost.org/doc/libs/1_75_0/doc/html/boost_asio.html)
- [boost.beast](https://github.com/boostorg/beast)
- [mio](https://github.com/tokio-rs/mio)

![](https://c.tenor.com/OTDlqAguqpEAAAAi/sonic-running.gif)
