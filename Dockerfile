FROM golang:1.14-alpine
WORKDIR /go/src/github.com/perlw/pict
ADD ./ ./
ADD ./valkyr.ini /app/valkyr.ini
RUN go build -o /app/valkyr

FROM alpine:latest
EXPOSE 80
EXPOSE 443
COPY --from=0 /app/valkyr /app/valkyr
COPY --from=0 /app/valkyr.ini /app/valkyr.ini
WORKDIR /app
CMD ["./valkyr"]
