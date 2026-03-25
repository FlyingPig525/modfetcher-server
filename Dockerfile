FROM golang:latest
LABEL authors="FlyingPig525"

WORKDIR /server

COPY go.mod go.sum ./
RUN go get -u all
RUN go mod download

COPY . .
RUN go build github.com/FlyingPig525/modfetcher-server

EXPOSE 80
CMD ["./modfetcher-server"]