FROM debian:bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    g++ \
    libgrpc++-dev \
    libprotobuf-dev \
    make \
    protobuf-compiler \
    protobuf-compiler-grpc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY api ./api
COPY engine_cpp ./engine_cpp
RUN cmake -S engine_cpp -B build -DCMAKE_BUILD_TYPE=Release \
    && cmake --build build --parallel 2 \
    && ctest --test-dir build --output-on-failure

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgrpc++1.51 \
    libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/build/catforge-game-engine /usr/local/bin/catforge-game-engine

EXPOSE 50051
USER 65532:65532
ENTRYPOINT ["catforge-game-engine", "0.0.0.0:50051"]
