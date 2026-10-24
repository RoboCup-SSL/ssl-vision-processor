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

if (WITH_SPINNAKER)

    find_path(SPINNAKER_INCLUDE_DIRS NAMES
            Spinnaker.h
            HINTS
            /opt/spinnaker/include/
            /usr/include/spinnaker/
            /usr/local/include/spinnaker/
    )

    find_library(SPINNAKER_LIBS NAMES
            Spinnaker
            HINTS
            /opt/spinnaker/lib
            /usr/lib
            /usr/local/lib
    )

    if (NOT (
            SPINNAKER_LIBS STREQUAL "SPINNAKER_LIBS-NOTFOUND" OR
            SPINNAKER_INCLUDE_DIRS STREQUAL "SPINNAKER_INCLUDE_DIRS-NOTFOUND"
    ))
        add_definitions( -DSPINNAKER_AVAILABLE )

        # This fixes some problems with paths after Ubuntu LTS upgrades and
        # on Nix. This does prevent overriding spinnaker.so via LD_LIBRARY_PATH.
        add_link_options( -Wl,--disable-new-dtags )

        # If system libusb isn't correctly on the path, spinnaker init function
        # throws an exception, and it's not clear why (their fault). Error here.
        find_library(LIBUSB_LIB NAMES usb-1.0 libusb-1.0.so.0)
        if (NOT LIBUSB_LIB)
            message(FATAL_ERROR "libusb-1.0 not found; required by the Spinnaker GenTL producer.")
        endif ()
        list(APPEND SPINNAKER_LIBS ${LIBUSB_LIB})
        
        message(STATUS "Spinnaker found, activating Spinnaker support.")
    else()
        set(SPINNAKER_INCLUDE_DIRS "")
        set(SPINNAKER_LIBS "")
    endif ()

else()
    set(SPINNAKER_INCLUDE_DIRS "")
    set(SPINNAKER_LIBS "")
endif (WITH_SPINNAKER)