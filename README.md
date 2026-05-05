# MP3 — Distributed Transactions

Two-phase locking (2PL) for isolation and two-phase commit (2PC) for atomicity across branch servers.

## Build

```bash
make
```

Produces `mp3/server` and `mp3/client`.

## Config file

Each line names a branch and its host/port:

```
A sp26-cs425-3701.cs.illinois.edu 1234
B sp26-cs425-3702.cs.illinois.edu 1234
C sp26-cs425-3703.cs.illinois.edu 1234
D sp26-cs425-3704.cs.illinois.edu 1234
E sp26-cs425-3705.cs.illinois.edu 1234
```

## Running a server

```bash
./mp3/server <branch> <config>
# e.g.
./mp3/server A config.txt
```

Start one process per branch. Each server listens on the port from the config file.

## Running the client

```bash
./mp3/client <id> <config>
# e.g.
./mp3/client foo config.txt
```

The client connects to a randomly selected server as coordinator and reads commands from stdin.

### Client commands

```
BEGIN
DEPOSIT <branch>.<account> <amount>
WITHDRAW <branch>.<account> <amount>
BALANCE <branch>.<account>
COMMIT
ABORT
```

## Local test

```bash
cd sample_test
./run.sh
```

Starts five servers on ports 10001–10005, runs two test cases, and diffs output against expected results.
