FROM golang:1.23.2-alpine AS build

WORKDIR /go/src/github.com/segmentio/chamber
COPY . .

ARG TARGETARCH
ARG VERSION
RUN test -n "${VERSION}"

RUN apk add -U make ca-certificates
RUN make linux VERSION=${VERSION} TARGETARCH=${TARGETARCH}

FROM scratch AS run

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/src/github.com/segmentio/chamber/chamber /chamber

ENTRYPOINT ["/chamber"]
