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
};

} // namespace nlohmann
