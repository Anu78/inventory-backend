FROM golang:alpine

WORKDIR /app

COPY . . 

RUN go build -o inventory-backend

EXPOSE 8888

CMD ["./inventory-backend"]