FROM golang:1.13-alpine AS build

WORKDIR /go/src/github.com/segmentio/chamber
COPY . .

ARG VERSION
RUN test -n "${VERSION}"

RUN apk add -U make
RUN make linux VERSION=${VERSION}
RUN apk --no-cache add ca-certificates

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/src/github.com/segmentio/chamber/chamber /chamber

ENTRYPOINT ["/chamber"]
