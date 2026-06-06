FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY cpa-dashboard/cpa-dashboard /app/cpa-dashboard

RUN chmod +x /app/cpa-dashboard

ENV TZ=Asia/Shanghai

CMD ["/app/cpa-dashboard"]
