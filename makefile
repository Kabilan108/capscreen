build/capscreen: $(shell find . -name '*.go')
	GO111MODULE=on go build -o build/capscreen .

build: build/capscreen

install:
	GO111MODULE=on go install

deps:
	GO111MODULE=on go mod tidy

clean:
	rm -f build/capscreen

run: build
	./build/capscreen
