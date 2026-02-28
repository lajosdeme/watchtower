.PHONY: build clean run test docker-build docker-run

BINARY_NAME=watchtower
DOCKER_IMAGE=watchtower

build:
	go build -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)

run: build
	./$(BINARY_NAME)

test:
	go test ./...

docker-build:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run --rm -it $(DOCKER_IMAGE)
