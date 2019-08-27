FROM golang:latest

WORKDIR /app

COPY main.go utils/ pkg/ indexes/ etc/ ./

RUN go mod download

COPY . .

RUN go build -o elktools .

# Set variables and config

ENTRYPOINT ["./elktools"]
