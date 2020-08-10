FROM golang:1.14-alpine
WORKDIR /go/src/github.com/perlw/pict
ADD ./ ./
ADD ./web/static /app/static
ADD ./web/template /app/template
RUN go build -o /app/pict ./cmd/pict

FROM alpine:latest
EXPOSE 80
COPY --from=0 /app/pict /app/pict
COPY --from=0 /app/static /app/static
COPY --from=0 /app/template /app/template
WORKDIR /app
CMD ["./pict"]
