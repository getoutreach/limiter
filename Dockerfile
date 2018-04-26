FROM golang
COPY .  /go/src/github.com/getoutreach/limiter
WORKDIR /go/src/github.com/getoutreach/limiter
RUN go install

FROM scratch
COPY --from=0 /go/bin/limiter /
ENTRYPOINT ["/limiter"]
CMD ["--help"]
