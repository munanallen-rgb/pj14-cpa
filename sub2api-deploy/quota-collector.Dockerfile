FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY quota-collector/quota-collector /app/quota-collector

RUN chmod +x /app/quota-collector

ENV TZ=Asia/Shanghai

CMD ["/app/quota-collector"]
