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

/* Dumps a Spinnaker camera's GenICam node maps as JSON, together with the pixel format vision_processor's
 * Spinnaker driver selects on it (see src/driver/spinnakerfeatures.h).
 *
 * Strictly read-only: it never writes a node or executes a command, so it is safe to run against
 * shared (e.g. league) cameras. It does not start acquisition. A GigE camera can only be opened by
 * one process, so stop vision_processor first. */

#include <algorithm>
#include <charconv>
#include <cerrno>
#include <cmath>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <set>
#include <string>
#include <system_error>
#include <type_traits>
#include <vector>

#include "Spinnaker.h"
#include "driver/spinnakerfeatures.h"

using namespace Spinnaker::GenApi;

constexpr int64_t PROBE_FORMAT_VERSION = 1;


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
					if(static_cast<unsigned char>(c) < 0x20)
						out << "\\u" << std::hex << std::setw(4) << std::setfill('0') << static_cast<int>(c) << std::dec << std::setfill(' ');
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


static std::string str(const Spinnaker::GenICam::gcstring& s) {
	return s.c_str();
}

/** Access mode of a node; evaluating it can need register reads, which may fail. */
static std::string safeAccessName(INode* node, std::vector<std::string>* errors = nullptr) {
	try {
		return str(EAccessModeClass::ToString(node->GetAccessMode()));
	} catch (std::exception &e) {
		if(errors != nullptr)
			errors->push_back(std::string("access: ") + e.what());
		return "error";
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
template<typename Get>
static void field(JsonWriter& json, std::vector<std::string>& errors, const char* key, Get get) {
	try {
		const auto value = get();
		json.key(key).value(value);
	} catch (Spinnaker::Exception &e) {
		errors.push_back(std::string(key) + ": " + e.GetErrorMessage());
	} catch (std::exception &e) {
		// Some register nodes leak library internal exceptions (e.g. boost::bad_get)
		errors.push_back(std::string(key) + ": " + e.what());
	}
}

/** Value, range and unit of an Integer or Float node. */
template<typename Ptr>
static void writeNumber(JsonWriter& json, std::vector<std::string>& errors, Ptr p, bool readable) {
	if(readable)
		field(json, errors, "value", [&] { return p->GetValue(); });
	field(json, errors, "min", [&] { return p->GetMin(); });
	field(json, errors, "max", [&] { return p->GetMax(); });
	// Integers always have an increment, floats optionally
	bool hasInc = true;
	if constexpr (std::is_same_v<Ptr, CFloatPtr>)
		hasInc = p->HasInc();
	if(hasInc)
		field(json, errors, "inc", [&] { return p->GetInc(); });
	field(json, errors, "unit", [&] { return str(p->GetUnit()); });
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
	json.key("visibility").value(str(EVisibilityClass::ToString(node->GetVisibility())));
	json.key("display_name").value(str(node->GetDisplayName()));
	json.key("description").value(str(node->GetDescription()));

	// Commands are never executed, registers and ports are not decoded
	switch(type) {
		case intfIInteger:
			writeNumber(json, errors, CIntegerPtr(node), readable);
			break;
		case intfIFloat:
			writeNumber(json, errors, CFloatPtr(node), readable);
			break;
		case intfIBoolean: {
			CBooleanPtr p = node;
			if(readable)
				field(json, errors, "value", [&] { return p->GetValue(); });
			break;
		}
		case intfIString: {
			CStringPtr p = node;
			if(readable)
				field(json, errors, "value", [&] { return str(p->GetValue()); });
			break;
		}
		case intfIEnumeration: {
			CEnumerationPtr p = node;
			if(readable)
				field(json, errors, "value", [&] { return str(p->GetCurrentEntry()->GetSymbolic()); });

			// Entries are listed even when the enumeration itself is unavailable
			std::vector<std::pair<std::string, std::string>> entryList;
			try {
				NodeList_t entries;
				p->GetEntries(entries);
				for(CEnumEntryPtr entry : entries)
					entryList.emplace_back(str(entry->GetSymbolic()), safeAccessName(entry));
			} catch (std::exception &e) {
				errors.push_back(std::string("entries: ") + e.what());
			}

			json.key("entries").beginArray();
			for(const auto& [entryName, entryAccess] : entryList) {
				json.beginObject(true);
				json.key("name").value(entryName);
				json.key("access").value(entryAccess);
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
	} catch (std::exception &e) {
		std::cerr << "Skipping category " << path << ": " << e.what() << std::endl;
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

/** The pixel format the driver will select on this camera. */
static void writeDriverView(JsonWriter& json, INodeMap& nodeMap) {
	auto optional = [&](const char* key, const std::string& value) {
		json.key(key);
		if(value.empty())
			json.null();
		else
			json.value(value);
	};

	json.beginObject();
	json.key("pixel_format").beginObject();
	optional("selected", SpinnakerFeatures::findBayerFormat(nodeMap));
	optional("current", readString(nodeMap, "PixelFormat"));
	// With Raw8 the driver reads this after switching formats; this is the value in the current format
	optional("pixel_color_filter", readString(nodeMap, "PixelColorFilter"));
	json.endObject();
	json.endObject();
}


static void writeHeader(JsonWriter& json, const std::string& spinnakerVersion) {
	json.beginObject();
	json.key("probe_format").value(PROBE_FORMAT_VERSION);
	json.key("spinnaker_version").value(spinnakerVersion);
}

/** Runs write with a JSON writer on outputPath, or on stdout when it is empty. */
template<typename Write>
static int withOutput(const std::string& outputPath, Write write) {
	std::ofstream file;
	if(!outputPath.empty()) {
		file.open(outputPath);
		if(!file) {
			std::cerr << "Cannot write " << outputPath << ": " << std::generic_category().message(errno) << std::endl;
			return 1;
		}
	}

	JsonWriter json(outputPath.empty() ? std::cout : file);
	return write(json);
}

static int listCameras(const Spinnaker::CameraList& camList, const std::string& spinnakerVersion, bool requested) {
	std::cerr << "Spinnaker " << spinnakerVersion << ", " << camList.GetSize() << " camera(s)" << std::endl;
	for(unsigned int i = 0; i < camList.GetSize(); i++) {
		INodeMap& tl = camList.GetByIndex(i)->GetTLDeviceNodeMap();
		std::cerr << "  [" << i << "] " << readString(tl, "DeviceModelName") << "  serial " << readString(tl, "DeviceSerialNumber")
				<< "  " << readString(tl, "DeviceType") << std::endl;
	}

	if(requested)
		return 0;
	std::cerr << (camList.GetSize() == 0 ? "No camera found" : "Several cameras found, select one with --serial or --index") << std::endl;
	return 1;
}

/** Interface node maps show e.g. each NIC's IP configuration, useful when a GigE camera is not found. */
static int dumpInterfaces(JsonWriter& json, Spinnaker::SystemPtr& system, const std::string& spinnakerVersion, bool all) {
	writeHeader(json, spinnakerVersion);
	json.key("system");
	writeNodeMap(json, system->GetTLNodeMap(), all);

	Spinnaker::InterfaceList interfaceList = system->GetInterfaces();
	json.key("interfaces").beginArray();
	for(unsigned int i = 0; i < interfaceList.GetSize(); i++)
		writeNodeMap(json, interfaceList.GetByIndex(i)->GetTLNodeMap(), all);
	json.endArray();
	interfaceList.Clear();

	json.endObject();
	return 0;
}

static int dumpCamera(JsonWriter& json, Spinnaker::CameraPtr& cam, const std::string& spinnakerVersion, bool all) {
	INodeMap& tlDevice = cam->GetTLDeviceNodeMap();
	writeHeader(json, spinnakerVersion);
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
		initError = e.GetErrorMessage();
	} catch (std::exception &e) {
		initError = e.what();
	}

	if(!initError.empty()) {
		json.endObject();
		json.key("error").value(initError);
		json.endObject();
		std::cerr << "Could not open camera: " << initError << std::endl;
		return 1;
	}

	json.key("tl_stream");
	writeNodeMap(json, cam->GetTLStreamNodeMap(), all);
	json.key("device");
	writeNodeMap(json, cam->GetNodeMap(), all);
	json.endObject();

	json.key("vision_processor");
	writeDriverView(json, cam->GetNodeMap());
	json.endObject();

	std::cerr << "Probed " << readString(tlDevice, "DeviceModelName") << " (serial " << readString(tlDevice, "DeviceSerialNumber")
			<< ", firmware " << readString(cam->GetNodeMap(), "DeviceFirmwareVersion") << ")" << std::endl;
	cam->DeInit();
	return 0;
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
			const std::string value = argv[++i];
			const auto [end, error] = std::from_chars(value.data(), value.data() + value.size(), index);
			if(error != std::errc() || end != value.data() + value.size() || index < 0) {
				std::cerr << "Invalid --index: " << value << std::endl;
				usage(argv[0]);
				return 2;
			}
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
	int status;

	if(interfaces) {
		status = withOutput(outputPath, [&](JsonWriter& json) { return dumpInterfaces(json, system, versionString, all); });
	} else if(list || (serial.empty() && index < 0 && camList.GetSize() != 1)) {
		status = listCameras(camList, versionString, list);
	} else {
		Spinnaker::CameraPtr cam;
		if(!serial.empty())
			cam = camList.GetBySerial(serial);
		else if(const auto i = static_cast<unsigned int>(std::max(index, 0)); i < camList.GetSize())
			cam = camList.GetByIndex(i); // Without --index there is exactly one camera
		if(!cam.IsValid()) {
			std::cerr << "Camera not found, see --list" << std::endl;
			status = 1;
		} else {
			status = withOutput(outputPath, [&](JsonWriter& json) { return dumpCamera(json, cam, versionString, all); });
		}
	}

	camList.Clear();
	system->ReleaseInstance();
	return status;
}
