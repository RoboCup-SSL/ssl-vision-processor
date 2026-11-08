#    Copyright 2024 Felix Weinmann
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.

cmake_minimum_required(VERSION 3.22)

# A build directory belongs to one toolchain: nix develop or the distro's
# packages. CMake caches the first one's compiler and library paths and would
# otherwise silently mix them with the other's; a build from the other one
# makes a binary that only runs in the first.
option(TOOLCHAIN_CHECK "Stop when a build directory is used from the toolchain that didn't configure it" ON)
if(TOOLCHAIN_CHECK AND DEFINED CACHE{CMAKE_CXX_COMPILER})
    set(TOOLCHAIN_COMPILER "$CACHE{CMAKE_CXX_COMPILER}")
    set(TOOLCHAIN_BUILD_DIR "${CMAKE_BINARY_DIR}")
    include("${CMAKE_SOURCE_DIR}/cmake/CheckToolchain.cmake")
endif()

project(vision_processor)
enable_language(ASM)

option(WITH_SPINNAKER "Build with Spinnaker support (if library available)" ON)
option(WITH_MVIMPACT "Build with mvIMPACT/Bluefox support (if library available)" ON)
option(WITH_DC1394 "Build with DC1394 (IIDC FireWire) support (if library available)" ON)
option(NATIVE_ARCH "Optimize Release builds for this machine's CPU (-march=native); the binary may not run on others" ON)

# Adapted from https://stackoverflow.com/a/48832234 CC BY-SA 4.0 by einpoklum
if (NOT CMAKE_BUILD_TYPE OR CMAKE_BUILD_TYPE STREQUAL "")
    set(CMAKE_BUILD_TYPE "RelWithDebInfo" CACHE STRING "" FORCE)
endif()

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_FLAGS_RELEASE "-O3 -ftree-vectorize -ffast-math")
if(NATIVE_ARCH)
    string(APPEND CMAKE_CXX_FLAGS_RELEASE " -march=native")
endif()
add_compile_options("-Wall")
list(APPEND CMAKE_MODULE_PATH ${PROJECT_SOURCE_DIR}/cmake)

find_package(PkgConfig REQUIRED)
pkg_check_modules(LIBAV REQUIRED IMPORTED_TARGET libavformat libavcodec libavutil)

find_package(Eigen3 REQUIRED)
find_package(mvIMPACT)
find_package(DC1394)
find_package(OpenCL REQUIRED)
find_package(OpenCV REQUIRED COMPONENTS core imgproc imgcodecs videoio)
find_package(Spinnaker)
find_package(yaml-cpp REQUIRED)

# Protobuf links to Abseil starting with 4.22.0. The FindProtobuf module does
# not yet account for this, so we need to use Config mode instead. However,
# Protobuf does not provide a Config CMake script on old versions, so we need
# to fallback to Module mode in that case.
find_package(Protobuf CONFIG QUIET)
if (Protobuf_FOUND)
    message(STATUS "Found Protobuf ${Protobuf_VERSION} via Config mode")
else()
    find_package(Protobuf REQUIRED)
    message(STATUS "Found Protobuf ${Protobuf_VERSION} via Module mode")
endif()


add_compile_definitions(CL_HPP_TARGET_OPENCL_VERSION=300)
# m (c math) and stdc++ (c++ standard library) explicitly required by LLD
link_libraries("m" "stdc++" ${YAML_CPP_LIBRARIES} ${OpenCV_LIBS} PkgConfig::LIBAV ${SPINNAKER_LIBS} OpenCL::OpenCL protobuf::libprotobuf ${mvIMPACT_LIBS} ${DC1394_LIBRARIES})
include_directories(SYSTEM ${YAML_INCLUDE_DIRS} ${OpenCV_INCLUDE_DIRS} ${SPINNAKER_INCLUDE_DIRS} Eigen3::Eigen ${mvIMPACT_INCLUDE_DIRS} ${DC1394_INCLUDE_DIRS})

include_directories(src)

# Generated code lives in the build directory, so each build directory (one per
# toolchain) has its own: protoc's output only compiles against the protobuf
# headers of the protoc that wrote it.
set(GENERATED_DIR "${CMAKE_BINARY_DIR}/generated")
set(PROTOC_OUT_DIR "${GENERATED_DIR}/proto")
include_directories("${GENERATED_DIR}" "${PROTOC_OUT_DIR}")

# Older checkouts generated into src/, where a quoted #include from src/ would
# find these before the build directory's copies. They're gitignored output.
foreach(stale "src/proto" "src/cl_kernels.h" "src/cl_kernels.S")
    if(EXISTS "${CMAKE_SOURCE_DIR}/${stale}")
        file(REMOVE_RECURSE "${CMAKE_SOURCE_DIR}/${stale}")
        message(STATUS "Removed generated ${stale}, now generated in ${GENERATED_DIR}")
    endif()
endforeach()

# league proto repository provides a generator function that fixes OUT_DIR limitations with protoc provided function
list(APPEND CMAKE_MODULE_PATH "${CMAKE_SOURCE_DIR}/proto/cmake")
include(SSLProtocolDefs)
set(PROTO_FILES
        vision/ssl_vision_detection.proto
        vision/ssl_vision_geometry.proto
        vision/ssl_vision_wrapper.proto
        vision/ssl_vp_config.proto
        gamecontroller/ssl_gc_common.proto
        gamecontroller/ssl_gc_game_event.proto
        gamecontroller/ssl_gc_geometry.proto
        gamecontroller/ssl_gc_referee_message.proto
)
ssl_protocol_defs_generate_cpp(
        OUT_DIR "${PROTOC_OUT_DIR}"
        PROTOS ${PROTO_FILES}
        SOURCES_VAR PROTO_SRCS
        HEADERS_VAR PROTO_HDRS
)

file(GLOB CL_KERNELS RELATIVE "${CMAKE_SOURCE_DIR}" kernel/*.cl)
file(GLOB_RECURSE SRC src/*.cpp src/*.c)
list(REMOVE_ITEM SRC "${CMAKE_SOURCE_DIR}/src/main.cpp" "${CMAKE_SOURCE_DIR}/src/geometry_benchmark.cpp" "${CMAKE_SOURCE_DIR}/src/blob_benchmark.cpp")

# Adapted from https://stackoverflow.com/a/56006001 CC BY-SA 4.0 by Itay Grudev
# Adapted from https://stackoverflow.com/a/4910421 CC BY-SY 4.0 by John Ripley
set(CL_KERNEL_HEADER "//Autogenerated by CMake, edits will be overridden\n#pragma once\n")
set(CL_KERNEL_ASM ".section .rodata\n") # # Autogenerated by CMake, edits will be overridden\n
foreach(input_file IN LISTS CL_KERNELS)
    cmake_path(GET input_file FILENAME input_filename)
    string(REPLACE . _ input_file_cname "${input_file}")
    string(REPLACE / _ input_file_cname "${input_file_cname}")
    string(REPLACE . _ input_filename_cname "${input_filename}")

    list(APPEND CL_KERNEL_PATHS "${CMAKE_SOURCE_DIR}/${input_file}")
    string(APPEND CL_KERNEL_ASM ".global kernel_${input_filename_cname}\n.type kernel_${input_filename_cname}, @object\nkernel_${input_filename_cname}:\n\t.incbin \"${CMAKE_SOURCE_DIR}/${input_file}\"\n\t.byte 0\n\n")
    string(APPEND CL_KERNEL_HEADER "extern const char kernel_${input_filename_cname}[];\n")
endforeach()

# Don't override cl_kernels.h or cl_kernels.S if already up-to-date to prevent unnecessary recompilation
set(CL_KERNEL_H "${GENERATED_DIR}/cl_kernels.h")
set(CL_KERNEL_S "${GENERATED_DIR}/cl_kernels.S")

if(EXISTS "${CL_KERNEL_H}")
    file(READ "${CL_KERNEL_H}" CL_KERNEL_HEADER_OLD)
endif()
if(NOT "${CL_KERNEL_HEADER_OLD}" STREQUAL "${CL_KERNEL_HEADER}")
    file(WRITE "${CL_KERNEL_H}" "${CL_KERNEL_HEADER}")
    message(STATUS "(Re-)Generated cl_kernels.h")
endif()

if(EXISTS "${CL_KERNEL_S}")
    file(READ "${CL_KERNEL_S}" CL_KERNEL_ASM_OLD)
endif()
if(NOT "${CL_KERNEL_ASM_OLD}" STREQUAL "${CL_KERNEL_ASM}")
    file(WRITE "${CL_KERNEL_S}" "${CL_KERNEL_ASM}")
    message(STATUS "(Re-)Generated cl_kernels.S")
endif()

add_custom_target(AUTOGENERATE DEPENDS "${CL_KERNEL_S}" ${PROTO_SRCS} ${PROTO_HDRS})

# No outputs, so it runs on every build, before anything is generated or compiled.
if(TOOLCHAIN_CHECK)
    add_custom_target(CHECK_TOOLCHAIN
            COMMAND ${CMAKE_COMMAND} "-DTOOLCHAIN_COMPILER=${CMAKE_CXX_COMPILER}" "-DTOOLCHAIN_BUILD_DIR=${CMAKE_BINARY_DIR}"
                    -P "${CMAKE_SOURCE_DIR}/cmake/CheckToolchain.cmake"
            VERBATIM)
    add_dependencies(AUTOGENERATE CHECK_TOOLCHAIN)
endif()
set_property(SOURCE "${CL_KERNEL_S}" APPEND PROPERTY OBJECT_DEPENDS ${CL_KERNEL_PATHS})
list(APPEND SRC ${PROTO_SRCS} "${CL_KERNEL_S}")

add_executable(${PROJECT_NAME} ${SRC} "src/main.cpp")
add_executable("geometry_benchmark" ${SRC} "src/geometry_benchmark.cpp")
add_executable("blob_benchmark" ${SRC} "src/blob_benchmark.cpp")

add_dependencies(${PROJECT_NAME} AUTOGENERATE)
add_dependencies("geometry_benchmark" AUTOGENERATE)
add_dependencies("blob_benchmark" AUTOGENERATE)

# Read-only camera node map dump, see tools/spinnaker_probe.cpp
if (SPINNAKER_LIBS)
    add_executable("spinnaker_probe" "tools/spinnaker_probe.cpp")
endif ()

include(GNUInstallDirs)
install(TARGETS ${PROJECT_NAME} DESTINATION ${CMAKE_INSTALL_BINDIR})
