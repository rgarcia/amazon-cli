VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

PREFIX ?= /usr/local

.PHONY: build install uninstall test vet lint clean

build:
	go build -ldflags "-X github.com/rgarcia/amazon-cli/pkg/cmd.Version=$(VERSION)" -o amzn ./cmd/amzn

install: amzn
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 amzn $(DESTDIR)$(PREFIX)/bin/amzn

amzn: $(shell find . -name '*.go' -not -path './vendor/*')
	go build -ldflags "-X github.com/rgarcia/amazon-cli/pkg/cmd.Version=$(VERSION)" -o amzn ./cmd/amzn

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/amzn

test:
	go test ./... -v

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -f amzn
