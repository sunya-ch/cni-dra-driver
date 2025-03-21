FROM golang:1.23 as build

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-extldflags -static" -o cni-dra-driver ./cmd/cni-dra-driver

FROM alpine:3.21

COPY --from=build /app/cni-dra-driver .

CMD ["./cni-dra-driver", "run"]
