#!/bin/bash

GOMEMLIMIT=2750MiB GODEBUG=asyncpreemptoff=1 strace -e trace=memory ./recv -slow=false -iter=16384
