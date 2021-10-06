FROM golang:1.16-alpine
WORKDIR /src
ADD ./ ./
RUN go build -o /app/valkyr ./cmd/valkyr

FROM alpine:latest
EXPOSE 80
EXPOSE 443
EXPOSE 9090
COPY --from=0 /app/valkyr /app/valkyr
WORKDIR /app
CMD ["./valkyr"]
