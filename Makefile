.PHONY: build test clean vet

build:
	go build -o mhist .

test:
	go test ./... -v -count=1

clean:
	rm -f mhist

vet:
	go vet ./...
