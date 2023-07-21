# docker build --tag=esb2ha_test_ha:latest .
# docker run --rm -p 127.0.0.1:8123:8123 esb2ha_test_ha:latest

FROM ghcr.io/home-assistant/home-assistant:stable

COPY ./ha_config/configuration.yaml /config/
COPY ./ha_config/storage/* /config/.storage/
