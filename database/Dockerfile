# PostgreSQL for the benchmark. The version is pinned so every run uses the same
# server build; the settings below are applied identically for all three APIs.
FROM postgres:18.1

ENV TZ=UTC \
    PGTZ=UTC

# shared_buffers: large enough to hold the 100k-row table and its indexes in memory.
# max_connections: headroom for three server pools plus admin sessions.
CMD ["postgres", \
     "-c", "shared_buffers=256MB", \
     "-c", "max_connections=200", \
     "-c", "timezone=UTC", \
     "-c", "log_min_duration_statement=-1"]
