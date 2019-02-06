FROM golang:1.11-alpine AS build

WORKDIR /go/src/github.com/segmentio/chamber
COPY . .

RUN apk add -U make git
RUN make linux

FROM scratch AS run

COPY --from=build /go/src/github.com/segmentio/chamber/chamber /chamber

ENTRYPOINT ["/chamber"]
