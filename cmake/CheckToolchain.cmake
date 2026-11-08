# Stops a build directory being used from the toolchain that didn't configure
# it: nix develop's or the distro's packages. CMakeLists.txt includes this when
# reconfiguring, and runs it with cmake -P before every build.
#
# Expects TOOLCHAIN_COMPILER (the build directory's C++ compiler) and
# TOOLCHAIN_BUILD_DIR.

string(FIND "${TOOLCHAIN_COMPILER}" "/nix/store/" _toolchain_nix_compiler)

if(DEFINED ENV{IN_NIX_SHELL} AND _toolchain_nix_compiler EQUAL -1)
    message(FATAL_ERROR "You're in nix develop, but ${TOOLCHAIN_BUILD_DIR} was configured outside it. "
            "Leave nix develop to build it, or delete it and configure it again here.")
elseif(NOT DEFINED ENV{IN_NIX_SHELL} AND NOT _toolchain_nix_compiler EQUAL -1)
    message(FATAL_ERROR "You're not in nix develop, but ${TOOLCHAIN_BUILD_DIR} was configured inside it. "
            "Enter nix develop to build it, or delete it and configure it again here.")
endif()
