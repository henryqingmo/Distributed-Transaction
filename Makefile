all: mp3/server mp3/client

mp3/server: cmd/server/main.go $(shell find internal -name '*.go')
	mkdir -p mp3
	go build -o mp3/server ./cmd/server

mp3/client: cmd/client/main.go $(shell find internal -name '*.go')
	mkdir -p mp3
	go build -o mp3/client ./cmd/client

clean:
	rm -f mp3/server mp3/client
