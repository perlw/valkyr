FROM golang:1.20-alpine
WORKDIR /src
ADD ./ ./
RUN go build -o /app/valkyr ./cmd/valkyr

FROM alpine:latest
EXPOSE 80
EXPOSE 443
EXPOSE 9090
ARG build_date
ENV BUILD_DATE=$build_date
COPY --from=0 /app/valkyr /app/valkyr
WORKDIR /app
CMD ["./valkyr"]
