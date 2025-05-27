build/capscreen: $(shell find . -name '*.go')
	go build -ldflags="-s -w" -o build/capscreen .

build/capscreen-linux-amd64: $(shell find . -name '*.go')
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/capscreen-linux-amd64 .

build: build/capscreen

install:
	go install

deps:
	go mod tidy

clean:
	rm -f build/capscreen
	rm -rf capscreen-linux-amd64
	rm -f capscreen-linux-amd64.tar.gz

run: build
	./build/capscreen

release: build/capscreen-linux-amd64
	cp build/capscreen-linux-amd64 capscreen
	tar czf capscreen-linux-amd64.tar.gz -C build capscreen
	rm -rf capscreen
