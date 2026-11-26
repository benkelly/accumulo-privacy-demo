FROM golang:1.27.4-alpine3.24 AS build
WORKDIR /src
COPY go.mod .
COPY *.go .
RUN CGO_ENABLED=0 go build -o /api .

FROM alpine:3.24.4
COPY --from=build /api /usr/local/bin/api
USER nobody
CMD ["api"]
