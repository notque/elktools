FROM golang:latest

WORKDIR /app

COPY go.sum go.mod main.go utils/ pkg/ indexes/ etc/ ./

COPY etc/ indexes/ pkg/ utils/ /app/
COPY go.mod go.sum main.go /app/

ENV GO111MODULE=on

#COPY . .

RUN go build -o elktools .

# Set variables and config

ENTRYPOINT ["./elktools"]
