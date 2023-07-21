# docker build --tag=esb2ha:latest .
# docker run --rm esb2ha:latest help download
FROM golang:alpine as build

COPY ./src/ /tmp/src
RUN cd /tmp/src && go build github.com/lorentz83/esb2ha

FROM alpine:latest as main

RUN apk add --no-cache tzdata

COPY --from=build /tmp/src/esb2ha /bin
COPY testing/esb2ha-entrypoint.sh /bin/entrypoint.sh

ENTRYPOINT ["/bin/entrypoint.sh"]

CMD ["/bin/esb2ha"]
