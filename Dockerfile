FROM golang:latest

WORKDIR /app

COPY etc/ vendor/ indexes/ pkg/ utils/ /app/
COPY go.mod go.sum main.go /app/

ENV GO111MODULE=on

RUN go build -o elktools .

ENTRYPOINT ["./elktools"]
