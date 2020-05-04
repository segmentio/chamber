FROM golang:1.13-alpine AS build
WORKDIR /go/src/github.com/segmentio/chamber
RUN apk add -U make git
RUN make linux
FROM golang:1.14-alpine as builder
RUN apk add --update curl ca-certificates make git gcc g++ python
# Enable go modules
ENV GO111MODULE=on
COPY . .
# this is an auto-generated build command
# based upon the first argument of the entrypoint in the existing dockerfile.  
# This will work in most cases, but it is important to note
# that in some situations you may need to define a different build output with the -o flag
# This comment may be safely removed
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s -extldflags "-static"' -o /chamber
FROM scratch AS run
COPY --from=build /go/src/github.com/segmentio/chamber/chamber /chamber
COPY --from=builder /chamber /chamber
ENTRYPOINT ["/chamber"]
