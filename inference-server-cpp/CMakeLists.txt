cmake_minimum_required(VERSION 3.10)

project(InferenceServer VERSION 1.0)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED True)

# Reverting to your global include setting
include_directories(include)

find_package(Protobuf REQUIRED)
find_package(gRPC CONFIG REQUIRED)
find_package(onnxruntime CONFIG REQUIRED)
find_package(Crow CONFIG REQUIRED)

set(PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/proto")
set(PROTO_FILE "${PROTO_DIR}/inference.proto")
set(ADVISOR_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../proto")
set(ADVISOR_PROTO_FILE "${ADVISOR_PROTO_DIR}/advisor.proto")

set(PROTO_SRCS 
    "${CMAKE_CURRENT_BINARY_DIR}/inference.pb.cc" 
    "${CMAKE_CURRENT_BINARY_DIR}/inference.grpc.pb.cc"
)
set(PROTO_HDRS 
    "${CMAKE_CURRENT_BINARY_DIR}/inference.pb.h" 
    "${CMAKE_CURRENT_BINARY_DIR}/inference.grpc.pb.h"
)

set(ADVISOR_SRCS 
    "${CMAKE_CURRENT_BINARY_DIR}/advisor.pb.cc" 
    "${CMAKE_CURRENT_BINARY_DIR}/advisor.grpc.pb.cc"
)
set(ADVISOR_HDRS 
    "${CMAKE_CURRENT_BINARY_DIR}/advisor.pb.h" 
    "${CMAKE_CURRENT_BINARY_DIR}/advisor.grpc.pb.h"
)

add_custom_command(
    OUTPUT ${PROTO_SRCS} ${PROTO_HDRS}
    COMMAND protobuf::protoc
    ARGS --proto_path="${PROTO_DIR}"
         --cpp_out="${CMAKE_CURRENT_BINARY_DIR}"
         --grpc_out="${CMAKE_CURRENT_BINARY_DIR}"
         --plugin=protoc-gen-grpc=$<TARGET_FILE:gRPC::grpc_cpp_plugin>
         "${PROTO_FILE}"
    DEPENDS "${PROTO_FILE}"
)

add_custom_command(
    OUTPUT ${ADVISOR_SRCS} ${ADVISOR_HDRS}
    COMMAND protobuf::protoc
    ARGS --proto_path="${ADVISOR_PROTO_DIR}"
         --cpp_out="${CMAKE_CURRENT_BINARY_DIR}"
         --grpc_out="${CMAKE_CURRENT_BINARY_DIR}"
         --plugin=protoc-gen-grpc=$<TARGET_FILE:gRPC::grpc_cpp_plugin>
         "${ADVISOR_PROTO_FILE}"
    DEPENDS "${ADVISOR_PROTO_FILE}"
)

# Your original GLOB picks up main.cpp and tokenizer.cpp
file(GLOB SOURCES "src/*.cpp")

add_executable(server_engine ${SOURCES} ${PROTO_SRCS} ${ADVISOR_SRCS})

# Fixed: Adding 'src' so main.cpp finds tokenizer.h
target_include_directories(server_engine PRIVATE 
    src 
    ${CMAKE_CURRENT_BINARY_DIR}
)

# The specific fix for onnxruntime_cxx_api.h
# This forces CMake to pull the header path directly from the package
if(TARGET onnxruntime::onnxruntime)
    get_target_property(ONNX_INC onnxruntime::onnxruntime INTERFACE_INCLUDE_DIRECTORIES)
    target_include_directories(server_engine PRIVATE ${ONNX_INC})
endif()

target_link_libraries(server_engine PRIVATE 
    gRPC::grpc++ 
    protobuf::libprotobuf 
    onnxruntime::onnxruntime 
    Crow::Crow
)