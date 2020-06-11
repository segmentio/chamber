FROM golang:1.13-alpine AS build

WORKDIR /go/src/github.com/segmentio/chamber
COPY . .

ARG VERSION
RUN test -n "${VERSION}"

RUN apk add -U make
RUN make linux VERSION=${VERSION}

FROM scratch AS run

COPY --from=build /go/src/github.com/segmentio/chamber/chamber /chamber

ENTRYPOINT ["/chamber"]
