BINARY := agentroute
PKG := ./cmd/agentroute

.PHONY: build test lint vet fmt tidy run demo clean

build:
	go build -o bin/$(BINARY) $(PKG)

run: build
	./bin/$(BINARY)

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	gofmt -l .

tidy:
	go mod tidy

demo:
	mkdir -p docs/demo
	go build -o bin/$(BINARY) $(PKG)
	PATH="$(PWD)/bin:$$PATH" vhs tapes/dashboard.tape
	PATH="$(PWD)/bin:$$PATH" vhs tapes/up.tape
	PATH="$(PWD)/bin:$$PATH" vhs tapes/model-picker.tape

clean:
	rm -rf bin/ dist/
