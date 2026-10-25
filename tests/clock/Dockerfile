# No shell, zoneinfo files, or system timezone database: Go must use embedded tzdata.
FROM scratch
COPY flare /flare
COPY config.yml /config.yml
ENV TZ=America/New_York FLARE_OFFLINE=true FLARE_DISABLE_LOGIN=true FLARE_PORT=5005
EXPOSE 5005
ENTRYPOINT ["/flare"]
