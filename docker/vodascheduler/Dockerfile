FROM golang:1.13.15

WORKDIR /vodascheduler
COPY . .
RUN mkdir -p /logs
WORKDIR ./pkg

RUN go build -o vodascheduler
CMD ["./vodascheduler"]