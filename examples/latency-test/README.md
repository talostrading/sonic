This comes in handy especially when comparing multiple types of executors but also to just establish once and for all
that we are faster than go.net.

# Methods

2 variants:

- echo server timestamps itself and produces histograms
    - upsides: stats for multiple clients
    - downsides: either we share the histogram for go.net which causes contention, or we make one histogram per
      connection in both echo servers

- echo server just replies:
    - upsides: super simple, we really measure the efficiency of the IO executors here
    - downsides: when connecting to multiple clients, each client needs to keep its own histogram

# Other stuff to test

- writing 8, 32, 64, 128, 256, 512, 1024, 2048, 4096 and 32KB
- keep track of number of messages

- examples: https://github.com/frevib/io_uring-echo-server/blob/master/benchmarks/benchmarks.md
