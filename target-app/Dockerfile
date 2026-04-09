
FROM golang:1.26-alpine


WORKDIR /app


COPY go.mod ./


COPY main.go ./


RUN CGO_ENABLED=0 go build -o chaos-app main.go


EXPOSE 8080

CMD ["./chaos-app"]