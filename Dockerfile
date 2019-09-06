FROM golang:latest AS build

WORKDIR /app

COPY . .

ENV GO111MODULE=on

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /bin/elktools .

FROM alpine

COPY --from=build /bin/elktools /bin/

ENTRYPOINT ["/bin/elktools"]
