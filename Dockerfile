FROM golang:latest

WORKDIR /app

COPY go.sum go.mod main.go utils/ pkg/ indexes/ etc/ ./

RUN go mod download

RUN go build -o elktools .

# Set variables and config

ENTRYPOINT ["./elktools"]
