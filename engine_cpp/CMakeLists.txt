cmake_minimum_required(VERSION 3.20)

project(catforge_game_engine LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(Protobuf REQUIRED)
find_package(gRPC CONFIG REQUIRED)

set(PROTO_FILE "${CMAKE_CURRENT_SOURCE_DIR}/../api/gameengine/v1/game_engine.proto")
set(PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../api")
set(GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")
set(GENERATED_BASE "${GENERATED_DIR}/gameengine/v1/game_engine")

add_custom_command(
    OUTPUT
        "${GENERATED_BASE}.pb.cc"
        "${GENERATED_BASE}.pb.h"
        "${GENERATED_BASE}.grpc.pb.cc"
        "${GENERATED_BASE}.grpc.pb.h"
    COMMAND ${CMAKE_COMMAND} -E make_directory "${GENERATED_DIR}"
    COMMAND ${Protobuf_PROTOC_EXECUTABLE}
        --proto_path=${PROTO_ROOT}
        --cpp_out=${GENERATED_DIR}
        --grpc_out=${GENERATED_DIR}
        --plugin=protoc-gen-grpc=$<TARGET_FILE:gRPC::grpc_cpp_plugin>
        "${PROTO_FILE}"
    DEPENDS "${PROTO_FILE}"
    VERBATIM
)

add_library(catforge_engine_proto
    "${GENERATED_BASE}.pb.cc"
    "${GENERATED_BASE}.grpc.pb.cc"
)
target_include_directories(catforge_engine_proto PUBLIC "${GENERATED_DIR}")
target_link_libraries(catforge_engine_proto PUBLIC protobuf::libprotobuf gRPC::grpc++)

add_library(catforge_engine_core src/game_engine.cpp)
target_include_directories(catforge_engine_core PUBLIC include)
target_compile_options(catforge_engine_core PRIVATE -Wall -Wextra -Wpedantic -Werror -Wno-missing-field-initializers)

add_executable(catforge-game-engine
    src/grpc_service.cpp
    src/main.cpp
)
target_link_libraries(catforge-game-engine PRIVATE catforge_engine_core catforge_engine_proto)
target_compile_options(catforge-game-engine PRIVATE -Wall -Wextra -Wpedantic -Werror -Wno-missing-field-initializers)

enable_testing()
add_executable(catforge_engine_tests tests/game_engine_test.cpp)
target_link_libraries(catforge_engine_tests PRIVATE catforge_engine_core)
target_compile_options(catforge_engine_tests PRIVATE -Wall -Wextra -Wpedantic -Werror -Wno-missing-field-initializers)
add_test(NAME catforge_engine_tests COMMAND catforge_engine_tests)

add_executable(catforge_feline_tests tests/feline_progression_test.cpp)
target_link_libraries(catforge_feline_tests PRIVATE catforge_engine_core)
target_compile_options(catforge_feline_tests PRIVATE -Wall -Wextra -Wpedantic -Werror -Wno-missing-field-initializers)
add_test(NAME catforge_feline_tests COMMAND catforge_feline_tests)
