FROM golang:1.22
WORKDIR /src
COPY . .
RUN go build -o /api .
CMD ["/api"]
