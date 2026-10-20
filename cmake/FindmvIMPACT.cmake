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

if (WITH_MVIMPACT)

    find_path(mvIMPACT_INCLUDE_DIRS NAMES
            mvIMPACT_CPP/mvIMPACT_acquire.h
            HINTS
            /opt/mvIMPACT_Acquire/
    )

    find_library(mvIMPACT_LIBS NAMES
            mvDeviceManager
            HINTS
            /opt/mvIMPACT_Acquire/lib/x86_64
    )

    if (NOT (
            mvIMPACT_LIBS STREQUAL "mvIMPACT_LIBS-NOTFOUND" OR
            mvIMPACT_INCLUDE_DIRS STREQUAL "mvIMPACT_INCLUDE_DIRS-NOTFOUND"
    ))
        add_definitions( -DMVIMPACT_AVAILABLE )
        message(STATUS "mvIMPACT found, activating mvIMPACT support.")
    else()
        set(mvIMPACT_INCLUDE_DIRS "")
        set(mvIMPACT_LIBS "")
    endif ()

else()
    set(mvIMPACT_INCLUDE_DIRS "")
    set(mvIMPACT_LIBS "")
endif (WITH_MVIMPACT)
