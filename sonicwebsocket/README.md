# notes
- use ioc.Post when calling the control callbacks
- separate the read and write part. Each should have their own (only-once) allocated frames, makes things easier

- some async operations can be paused (define the cases in which this actually happens)
  - in this case, save the handlers:
    - op_rd:          paused read op
    - op_wr:          paused write op
    - op_ping:        paused ping op
    - op_idle_ping:   paused idle ping op
    - op_close:       paused close op
    - op_r_rd:        paused read op (async read)
    - op_r_close:     paused close op (async read)
  - these handlers should at points be invoked while doing a read/write/ping/pong/close
  - maybe introduce maybe_invoke here
  - all these things should actually be slices not a single element

# to check
- status::closing
- how to do proper teardown 

- upcalling 

- (NOT IMPORTANT) need to to utf8 checking for text frames if client wants it 

# state machines
## read loop
- start
- check if soft mutex is locked. if not locked then continue
  - if locked, then we cannot continue this read operation, so we need to save it for later. Save it in op_r_rd and call maybe_invoke() later
- read frame header
  - if message too big, set the close code message too big and close
  - if frame is corrupt, set the close code to protocol error and close
- close.maybe_invoke() (everytime we call this, the reader releases the soft mutex) -- this is op_r_close. here goto do_suspend
- acquire soft mutex

- if it is a control frame
  - if frame is ping
    - close.maybe_invoke() -> like above, op_r_close
    - send pong
      - writer acquires soft mutex
      - write the pong
      - close.maybe_invoke (also op_idle, op_ping, op_wr maybe invoke for all 3)

  - if frame is pong
    - ignore pong if closing and continue closing sequence
    - just read it and call the callback then clear the buffer
    - maybe_invoke op_close op_idle_ping op_ping op_wr

  - if frame is close
    - set rd_close (assert it is not set before - can panic for the beginning)
    - read the close message, extract the code and reason and call the control callback in ioc's goroutine
    - if already state==closing then the state was changed by the writer who is trying to close (wr_close should be true then i.e. we closed during a write). In this case, close normally.

- if fin is not set but the frame is empty, then goto start

- if no compression used
  - if there some bytes remaining to be read in the current frame (bytes_remaining = frame_length - bytes_read) then go on and read the payload
  - (somehow merge with the above) if there is stuff in the pending buffer, then read that. IF NOT, read directly in the caller's buffer
      - if frame is text, check for utf8 is caller set it
  - assert that rd_done is not set
  - if rd_remain == 0 and the frame is FIN then set rd_done=true and go back to start

- if compression used (TODO and NOT IMPORTANT(for now))

### closing nicely during reads
  - writer acquires the soft mutex
  - set state to closing
  - if the writer did not already have wr_close set then
    - wr_close = true
    - serialize and send the close frame

## write loop
