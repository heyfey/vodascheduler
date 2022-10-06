FROM golang:1.18

WORKDIR /vodascheduler
COPY . .
WORKDIR ./pkg/scheduler

RUN go build -o sched
CMD ["./sched -gpu=default -resume"]