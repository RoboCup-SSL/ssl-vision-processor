/*
     Copyright 2026 Felix Weinmann

     Licensed under the Apache License, Version 2.0 (the "License");
     you may not use this file except in compliance with the License.
     You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

     Unless required by applicable law or agreed to in writing, software
     distributed under the License is distributed on an "AS IS" BASIS,
     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
     See the License for the specific language governing permissions and
     limitations under the License.
 */
#pragma once
#include <cstdlib>
#include <iostream>
#include <set>
#include <sstream>
#include <string>

#define VP_LOG(data) std::cout << "[" << __FILE__ << ":" << __LINE__ << "] " << data << std::endl
#define WARN(data) std::cerr << "[" << __FILE__ << ":" << __LINE__ << "] " << data << std::endl
#define FATAL(data) do { WARN(data); std::exit(1); } while(0)

inline std::set<std::string>& warnedMessages() {
	static std::set<std::string> messages;
	return messages;
}
// Warns only the first time a message occurs. The local's name avoids capturing a caller's variable used in data.
#define WARN_ONCE(data) do { std::ostringstream warnOnceMessage_; warnOnceMessage_ << data; \
	if(warnedMessages().insert(warnOnceMessage_.str()).second) WARN(warnOnceMessage_.str()); } while(0)
