/*
     Copyright 2026 Will Stuckey

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

/* Dumps a Spinnaker camera's GenICam node maps as JSON, together with the nodes vision_processor's
 * Spinnaker driver resolves on it (see src/driver/spinnakerfeatures.h).
 *
 * Strictly read-only: it never writes a node or executes a command, so it is safe to run against
 * shared (e.g. league) cameras. It does not start acquisition. A GigE camera can only be opened by
 * one process, so stop vision_processor first. */

#include <cmath>
#include <cstring>
#include <fstream>
#include <functional>
#include <iomanip>
#include <iostream>
#include <set>
#include <string>
#include <vector>

#include "Spinnaker.h"
#include "driver/spinnakerfeatures.h"

using namespace Spinnaker::GenApi;

static const int PROBE_FORMAT_VERSION = 1;


/** Minimal pretty-printing JSON writer. Compact containers keep small objects (enum entries) on one line. */
class JsonWriter {
public:
	explicit JsonWriter(std::ostream& out): out(out) {}

	JsonWriter& key(const std::string& name) {
		separator();
		string(name);
		out << ": ";
		afterKey = true;
		return *this;
	}

	void beginObject(bool compact = false) { open('{', compact); }
	void endObject() { close('}'); }
	void beginArray(bool compact = false) { open('[', compact); }
	void endArray() { close(']'); }

	void value(const std::string& v) { separator(); string(v); }
	void value(const char* v) { value(std::string(v)); }
	void value(bool v) { separator(); out << (v ? "true" : "false"); }
	void value(int64_t v) { separator(); out << v; }
	void value(double v) {
		separator();
		if(std::isfinite(v))
			out << std::setprecision(15) << v;
		else
			out << "null"; // JSON has no inf/nan
	}
	void null() { separator(); out << "null"; }

private:
	struct Level {
		bool first;
		bool compact;
	};

	void open(char c, bool compact) {
		separator();
		out << c;
		levels.push_back({true, compact || (!levels.empty() && levels.back().compact)});
	}

	void close(char c) {
		const Level level = levels.back();
		levels.pop_back();
		if(!level.first && !level.compact)
			newline();
		out << c;
		if(levels.empty())
			out << '\n';
	}

	void separator() {
		if(afterKey) {
			afterKey = false;
			return;
		}
		if(levels.empty())
			return;

		Level& level = levels.back();
		if(!level.first)
			out << ',';
		if(level.compact) {
			if(!level.first)
				out << ' ';
		} else {
			newline();
		}
		level.first = false;
	}

	void newline() {
		out << '\n' << std::string(levels.size() * 2, ' ');
	}

	void string(const std::string& s) {
		out << '"';
		for(const char c : s) {
			switch(c) {
				case '"': out << "\\\""; break;
				case '\\': out << "\\\\"; break;
				case '\n': out << "\\n"; break;
				case '\r': out << "\\r"; break;
				case '\t': out << "\\t"; break;
				default:
					if((unsigned char)c < 0x20)
						out << "\\u" << std::hex << std::setw(4) << std::setfill('0') << (int)c << std::dec << std::setfill(' ');
					else
						out << c;
			}
		}
		out << '"';
	}

	std::ostream& out;
	std::vector<Level> levels;
	bool afterKey = false;
};


static const char* accessName(EAccessMode mode) {
	switch(mode) {
		case NI: return "NI";
		case NA: return "NA";
		case WO: return "WO";
		case RO: return "RO";
		case RW: return "RW";
		default: return "undefined";
	}
}

/** Access mode of a node; evaluating it can need register reads, which may fail. */
static const char* safeAccessName(INode* node, std::vector<std::string>* errors = nullptr) {
	try {
		return accessName(node->GetAccessMode());
	} catch (std::exception &e) {
		if(errors != nullptr)
			errors->push_back(std::string("access: ") + e.what());
		return "error";
	}
}

static const char* visibilityName(EVisibility visibility) {
	switch(visibility) {
		case Beginner: return "Beginner";
		case Expert: return "Expert";
		case Guru: return "Guru";
		case Invisible: return "Invisible";
		default: return "undefined";
	}
}

static const char* typeName(EInterfaceType type) {
	switch(type) {
		case intfIValue: return "Value";
		case intfIBase: return "Base";
		case intfIInteger: return "Integer";
		case intfIBoolean: return "Boolean";
		case intfICommand: return "Command";
		case intfIFloat: return "Float";
		case intfIString: return "String";
		case intfIRegister: return "Register";
		case intfICategory: return "Category";
		case intfIEnumeration: return "Enumeration";
		case intfIEnumEntry: return "EnumEntry";
		case intfIPort: return "Port";
		default: return "undefined";
	}
}

/** Writes key: get() if the camera answers. Values are read before anything is written, so a node
 * that throws mid-read cannot leave half a field in the output. */
template<typename T>
static void field(JsonWriter& json, std::vector<std::string>& errors, const char* key, const std::function<T()>& get) {
	try {
		const T value = get();
		json.key(key).value(value);
	} catch (Spinnaker::Exception &e) {
		errors.push_back(std::string(key) + ": " + e.GetErrorMessage());
	} catch (std::exception &e) {
		// Some register nodes leak library internal exceptions (e.g. boost::bad_get)
		errors.push_back(std::string(key) + ": " + e.what());
	}
}

static std::string str(const Spinnaker::GenICam::gcstring& s) {
	return s.c_str();
}

static void writeNode(JsonWriter& json, INode* node, const std::string& category) {
	std::vector<std::string> errors;
	const EInterfaceType type = node->GetPrincipalInterfaceType();
	const std::string access = safeAccessName(node, &errors);
	const bool readable = access == "RO" || access == "RW";

	json.beginObject();
	json.key("name").value(str(node->GetName()));
	json.key("category").value(category);
	json.key("type").value(typeName(type));
	json.key("access").value(access);
	json.key("visibility").value(visibilityName(node->GetVisibility()));
	json.key("display_name").value(str(node->GetDisplayName()));
	json.key("description").value(str(node->GetDescription()));

	// Commands are never executed, registers and ports are not decoded
	switch(type) {
		case intfIInteger: {
			CIntegerPtr p = node;
			if(readable)
				field<int64_t>(json, errors, "value", [&] { return p->GetValue(); });
			field<int64_t>(json, errors, "min", [&] { return p->GetMin(); });
			field<int64_t>(json, errors, "max", [&] { return p->GetMax(); });
			field<int64_t>(json, errors, "inc", [&] { return p->GetInc(); });
			field<std::string>(json, errors, "unit", [&] { return str(p->GetUnit()); });
			break;
		}
		case intfIFloat: {
			CFloatPtr p = node;
			if(readable)
				field<double>(json, errors, "value", [&] { return p->GetValue(); });
			field<double>(json, errors, "min", [&] { return p->GetMin(); });
			field<double>(json, errors, "max", [&] { return p->GetMax(); });
			if(p->HasInc())
				field<double>(json, errors, "inc", [&] { return p->GetInc(); });
			field<std::string>(json, errors, "unit", [&] { return str(p->GetUnit()); });
			break;
		}
		case intfIBoolean: {
			CBooleanPtr p = node;
			if(readable)
				field<bool>(json, errors, "value", [&] { return p->GetValue(); });
			break;
		}
		case intfIString: {
			CStringPtr p = node;
			if(readable)
				field<std::string>(json, errors, "value", [&] { return str(p->GetValue()); });
			break;
		}
		case intfIEnumeration: {
			CEnumerationPtr p = node;
			if(readable)
				field<std::string>(json, errors, "value", [&] { return str(p->GetCurrentEntry()->GetSymbolic()); });

			// Entries are listed even when the enumeration itself is unavailable
			std::vector<std::pair<std::string, const char*>> entryList;
			try {
				NodeList_t entries;
				p->GetEntries(entries);
				for(CEnumEntryPtr entry : entries)
					entryList.emplace_back(str(entry->GetSymbolic()), safeAccessName(entry));
			} catch (std::exception &e) {
				errors.push_back(std::string("entries: ") + e.what());
			}

			json.key("entries").beginArray();
			for(const auto& [name, access] : entryList) {
				json.beginObject(true);
				json.key("name").value(name);
				json.key("access").value(access);
				json.endObject();
			}
			json.endArray();
			break;
		}
		default:
			break;
	}

	if(!errors.empty()) {
		json.key("errors").beginArray();
		for(const std::string& error : errors)
			json.value(error);
		json.endArray();
	}
	json.endObject();
}

static void walkCategory(JsonWriter& json, INode* node, const std::string& path, std::set<std::string>& visited) {
	CCategoryPtr category = node;
	FeatureList_t features;
	try {
		category->GetFeatures(features);
	} catch (Spinnaker::Exception &e) {
		std::cerr << "Skipping category " << path << ": " << e.GetErrorMessage() << std::endl;
		return;
	}

	for(IValue* feature : features) {
		INode* child = feature->GetNode();
		const std::string name = str(child->GetName());
		if(!visited.insert(name).second)
			continue;

		if(child->GetPrincipalInterfaceType() == intfICategory)
			walkCategory(json, child, path.empty() ? name : path + "/" + name, visited);
		else
			writeNode(json, child, path);
	}
}

/** Writes the nodes reachable through the category tree (what SpinView shows), plus with all=true every
 * other value node in the map (the driver looks nodes up by name, so these can matter too). */
static void writeNodeMap(JsonWriter& json, INodeMap& nodeMap, bool all) {
	std::set<std::string> visited;
	json.beginArray();

	INode* root = nodeMap.GetNode("Root");
	if(root != nullptr)
		walkCategory(json, root, "", visited);

	if(all) {
		NodeList_t nodes;
		nodeMap.GetNodes(nodes);
		for(INode* node : nodes) {
			const EInterfaceType type = node->GetPrincipalInterfaceType();
			if(type == intfICategory || type == intfIEnumEntry || type == intfIPort)
				continue;
			if(visited.insert(str(node->GetName())).second)
				writeNode(json, node, "");
		}
	}

	json.endArray();
}

static std::string readString(INodeMap& nodeMap, const char* name) {
	CNodePtr node = nodeMap.GetNode(name);
	try {
		if(!IsReadable(node))
			return "";
		return str(CValuePtr(node)->ToString());
	} catch (std::exception &e) {
		return "";
	}
}

/** The nodes the driver will resolve for each feature, with the same preference order. */
static void writeDriverView(JsonWriter& json, INodeMap& nodeMap) {
	json.beginObject();

	json.key("pixel_format").beginObject();
	const std::string selected = SpinnakerFeatures::findBayerFormat(nodeMap);
	json.key("selected");
	if(selected.empty())
		json.null();
	else
		json.value(selected);
	json.key("current");
	const std::string current = readString(nodeMap, "PixelFormat");
	if(current.empty())
		json.null();
	else
		json.value(current);
	// With Raw8 the driver reads this after switching formats; this is the value in the current format
	json.key("pixel_color_filter");
	const std::string colorFilter = readString(nodeMap, "PixelColorFilter");
	if(colorFilter.empty())
		json.null();
	else
		json.value(colorFilter);
	json.endObject();

	json.key("features").beginObject();
	for(const SpinnakerFeature* feature : SpinnakerFeatures::All) {
		json.key(feature->label).beginObject();

		std::string resolved;
		json.key("candidates").beginArray();
		for(const char* name : feature->nodes) {
			CNodePtr node = nodeMap.GetNode(name);
			json.beginObject(true);
			json.key("name").value(name);
			json.key("access").value(node.IsValid() ? safeAccessName(node) : "missing");
			json.endObject();

			if(resolved.empty() && SpinnakerFeatures::isAvailable(node))
				resolved = name;
		}
		json.endArray();

		json.key("resolved");
		if(resolved.empty())
			json.null();
		else
			json.value(resolved);
		json.endObject();
	}
	json.endObject();

	json.endObject();
}


static void usage(const char* argv0) {
	std::cerr << "Usage: " << argv0 << " [--list | --interfaces] [--serial SERIAL | --index N] [--all] [-o FILE]\n"
			<< "\n"
			<< "Dumps a Spinnaker camera's node maps as JSON (stdout unless -o is given). Read-only.\n"
			<< "  --list       List connected cameras and exit\n"
			<< "  --interfaces Dump the system and interface (NIC/USB) node maps instead of a camera\n"
			<< "  --serial S   Probe the camera with this serial number\n"
			<< "  --index N    Probe the camera at this enumeration index\n"
			<< "  --all        Also include value nodes outside the category tree\n"
			<< "  -o FILE      Write the JSON to FILE\n"
			<< "Without --serial or --index the only connected camera is probed.\n";
}

int main(int argc, char* argv[]) {
	bool list = false;
	bool interfaces = false;
	bool all = false;
	std::string serial;
	int index = -1;
	std::string outputPath;

	for(int i = 1; i < argc; i++) {
		const std::string arg = argv[i];
		const bool hasValue = i + 1 < argc;
		if(arg == "--list") {
			list = true;
		} else if(arg == "--interfaces") {
			interfaces = true;
		} else if(arg == "--all") {
			all = true;
		} else if(arg == "--serial" && hasValue) {
			serial = argv[++i];
		} else if(arg == "--index" && hasValue) {
			index = std::atoi(argv[++i]);
		} else if(arg == "-o" && hasValue) {
			outputPath = argv[++i];
		} else {
			usage(argv[0]);
			return arg == "-h" || arg == "--help" ? 0 : 2;
		}
	}

	Spinnaker::SystemPtr system = Spinnaker::System::GetInstance();
	const Spinnaker::LibraryVersion version = system->GetLibraryVersion();
	const std::string versionString = std::to_string(version.major) + "." + std::to_string(version.minor) + "." + std::to_string(version.type) + "." + std::to_string(version.build);

	Spinnaker::CameraList camList = system->GetCameras();
	int status = 0;

	if(interfaces) {
		std::ofstream file;
		if(!outputPath.empty())
			file.open(outputPath);
		if(!outputPath.empty() && !file) {
			std::cerr << "Cannot write " << outputPath << ": " << std::strerror(errno) << std::endl;
			status = 1;
		} else {
			// Interface node maps show e.g. each NIC's IP configuration, useful when a GigE camera is not found
			JsonWriter json(outputPath.empty() ? std::cout : file);
			json.beginObject();
			json.key("probe_format").value((int64_t)PROBE_FORMAT_VERSION);
			json.key("spinnaker_version").value(versionString);
			json.key("system");
			writeNodeMap(json, system->GetTLNodeMap(), all);

			Spinnaker::InterfaceList interfaceList = system->GetInterfaces();
			json.key("interfaces").beginArray();
			for(unsigned int i = 0; i < interfaceList.GetSize(); i++) {
				Spinnaker::InterfacePtr iface = interfaceList.GetByIndex(i);
				writeNodeMap(json, iface->GetTLNodeMap(), all);
			}
			json.endArray();
			interfaceList.Clear();
			json.endObject();
		}
	} else if(list || (serial.empty() && index < 0 && camList.GetSize() != 1)) {
		std::cerr << "Spinnaker " << versionString << ", " << camList.GetSize() << " camera(s)" << std::endl;
		for(unsigned int i = 0; i < camList.GetSize(); i++) {
			Spinnaker::CameraPtr cam = camList.GetByIndex(i);
			INodeMap& tl = cam->GetTLDeviceNodeMap();
			std::cerr << "  [" << i << "] " << readString(tl, "DeviceModelName") << "  serial " << readString(tl, "DeviceSerialNumber")
					<< "  " << readString(tl, "DeviceType") << std::endl;
		}
		if(!list) {
			std::cerr << (camList.GetSize() == 0 ? "No camera found" : "Several cameras found, select one with --serial or --index") << std::endl;
			status = 1;
		}
	} else {
		Spinnaker::CameraPtr cam;
		if(!serial.empty())
			cam = camList.GetBySerial(serial);
		else if(index >= 0 && (unsigned int)index < camList.GetSize())
			cam = camList.GetByIndex(index);
		else if(index < 0)
			cam = camList.GetByIndex(0);

		if(!cam.IsValid()) {
			std::cerr << "Camera not found, see --list" << std::endl;
			status = 1;
		} else {
			std::ofstream file;
			if(!outputPath.empty()) {
				file.open(outputPath);
				if(!file) {
					std::cerr << "Cannot write " << outputPath << ": " << std::strerror(errno) << std::endl;
					status = 1;
				}
			}

			if(status == 0) {
				JsonWriter json(outputPath.empty() ? std::cout : file);
				INodeMap& tlDevice = cam->GetTLDeviceNodeMap();

				json.beginObject();
				json.key("probe_format").value((int64_t)PROBE_FORMAT_VERSION);
				json.key("spinnaker_version").value(versionString);
				json.key("camera").beginObject();
				json.key("model").value(readString(tlDevice, "DeviceModelName"));
				json.key("serial").value(readString(tlDevice, "DeviceSerialNumber"));
				json.key("vendor").value(readString(tlDevice, "DeviceVendorName"));
				json.key("interface").value(readString(tlDevice, "DeviceType"));
				json.endObject();

				// The transport layer node maps are available without opening the camera
				json.key("node_maps").beginObject();
				json.key("tl_device");
				writeNodeMap(json, tlDevice, all);

				std::string initError;
				try {
					cam->Init();
				} catch (Spinnaker::Exception &e) {
					// Typically a GigE camera in another subnet or already opened by another process
					initError = e.GetFullErrorMessage();
				}

				if(initError.empty()) {
					json.key("tl_stream");
					writeNodeMap(json, cam->GetTLStreamNodeMap(), all);
					json.key("device");
					writeNodeMap(json, cam->GetNodeMap(), all);
					json.endObject();

					json.key("vision_processor");
					writeDriverView(json, cam->GetNodeMap());

					std::cerr << "Probed " << readString(tlDevice, "DeviceModelName") << " (serial " << readString(tlDevice, "DeviceSerialNumber")
							<< ", firmware " << readString(cam->GetNodeMap(), "DeviceFirmwareVersion") << ")" << std::endl;
					cam->DeInit();
				} else {
					json.endObject();
					json.key("error").value(initError);
					std::cerr << "Could not open camera: " << initError << std::endl;
					status = 1;
				}
				json.endObject();
			}
		}
	}

	camList.Clear();
	system->ReleaseInstance();
	return status;
}
