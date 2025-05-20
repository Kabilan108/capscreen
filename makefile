build:
	GO111MODULE=on go build -o capscreen .

install:
	GO111MODULE=on go install

deps:
	GO111MODULE=on go mod tidy

clean:
	rm -f capscreen

run: build
	./capscreen
