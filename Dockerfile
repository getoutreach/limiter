FROM golang
COPY .  /go/src/github.com/getoutreach/limiter
WORKDIR /go/src/github.com/getoutreach/limiter
RUN go get
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /limiter .

FROM alpine:3.7
COPY --from=0 /limiter /
ENTRYPOINT ["/limiter"]
