#pragma once

#include <map>
#include <string>
#include <variant>
#include <vector>

namespace nlohmann {

enum class value_t {
    null,
    object,
    array,
    string,
    boolean,
    number_integer
};

class json {
public:
    value_t type = value_t::null;
    std::map<std::string, json> object_val;
    std::string string_val;

    bool is_object() const noexcept {
        return type == value_t::object;
    }

    bool is_null() const noexcept {
        return type == value_t::null;
    }

    bool operator==(const json& other) const {
        if (type != other.type) return false;
        if (type == value_t::string) return string_val == other.string_val;
        if (type == value_t::object) return object_val == other.object_val;
        return true;
    }

    bool operator!=(const json& other) const {
        return !(*this == other);
    }

    static auto merge_diff(const json& source, const json& target) -> json {
        if (!target.is_object()) {
            return target;
        }

        json result;
        result.type = value_t::object;

        if (source.is_object()) {
            for (const auto& [key, val] : source.object_val) {
                auto it = target.object_val.find(key);
                if (it != target.object_val.end()) {
                    if (val != it->second) {
                        result.object_val[key] = merge_diff(val, it->second);
                    }
                } else {
                    json null_node;
                    null_node.type = value_t::null;
                    result.object_val[key] = null_node;
                }
            }
            for (const auto& [key, val] : target.object_val) {
                if (source.object_val.find(key) == source.object_val.end()) {
                    result.object_val[key] = val;
                }
            }
        } else {
            result = target;
        }

        return result;
    }
};

} // namespace nlohmann
