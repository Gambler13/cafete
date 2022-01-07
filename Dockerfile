FROM golang:latest as builder

WORKDIR /src

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
ENV GO111MODULE=on
RUN  GOOS=`go env GOHOSTOS` GOARCH=`go env GOHOSTARCH` go build -o cafete

FROM alpine:latest
RUN apk --no-cache add ca-certificates gcompat
WORKDIR /cafete/
COPY --from=builder /src/cafete .
COPY /static ./static
COPY index.html .
COPY contact.html .

ENTRYPOINT "/cafete/cafete"