FROM golang:1.18

WORKDIR /vodascheduler
COPY . .
WORKDIR ./pkg/allocator

RUN go build -o resource_allocator
CMD ["./resource_allocator"]