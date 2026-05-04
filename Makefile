VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
BINARY ?= amzn

.PHONY: build install uninstall test vet lint clean

build: $(BINARY)

install:
	test -x $(BINARY) || { echo "missing $(BINARY); run make build first"; exit 1; }
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

$(BINARY): $(shell find . -name '*.go' -not -path './vendor/*')
	go build -ldflags "-X github.com/rgarcia/amazon-cli/pkg/cmd.Version=$(VERSION)" -o $(BINARY) ./cmd/amzn

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

test:
	go test ./... -v

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
