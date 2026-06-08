FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY portal-api/portal-api /app/portal-api

RUN chmod +x /app/portal-api

ENV TZ=Asia/Shanghai

CMD ["/app/portal-api"]
