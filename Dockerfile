FROM golang:1.21-alpine AS build
COPY . /app
WORKDIR /app
RUN go build -o /gluetun-sync


FROM alpine:3.18
COPY --from=build /gluetun-sync /gluetun-sync
ENTRYPOINT [ "/gluetun-sync" ]