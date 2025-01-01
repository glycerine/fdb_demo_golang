fdb_demo_golang
===============

I'm given to understand from reading the FoundationDB
documentation that the key to getting acceptable
write throughput is to use many connections to the
database in parallel.

However: when I try to do this, I get consitent panics
of the "CGO received a signal" type.

See the two files herein: crash.log.txt and crash.log2.txt
for stack traces, and run go test -v in the ./kvfile
directory to reproduce it.

go version go1.23.3 linux/amd64


Report and further diagnosis at https://github.com/apple/foundationdb/issues/11856
