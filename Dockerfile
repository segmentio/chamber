FROM golang:1.10-alpine3.7 as builder

RUN apk --no-cache add make bash git

ARG gopath="/go"
ENV GOPATH=${gopath}
ENV PROJECT_DIR=$GOPATH/src/github.com/segmentio/chamber
WORKDIR $PROJECT_DIR

COPY . .

# ensure linux binaries are compatible
RUN CGO_ENABLED=0 make build

FROM builder as test
RUN make test

FROM alpine:3.7 as production

COPY --from=builder /go/src/github.com/segmentio/chamber/bin/chamber /

ENTRYPOINT ["/chamber"]

CMD ["help"]