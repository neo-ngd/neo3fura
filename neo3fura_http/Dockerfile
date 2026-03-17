FROM golang:1.21

ENV GO111MODULE="on"

ENV GOPROXY="https://goproxy.cn,direct"

ARG RT

RUN echo $RT

ENV RUNTIME=$RT

RUN mkdir application

COPY . ./application

WORKDIR "application"

RUN  go build -o main app/neo3fura/src.go

EXPOSE 1926

CMD ["./main"]
