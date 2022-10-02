FROM golang:1.18

WORKDIR /vodascheduler
COPY . .
WORKDIR ./pkg/service

RUN go build -o training_service
CMD ["./training_service"]