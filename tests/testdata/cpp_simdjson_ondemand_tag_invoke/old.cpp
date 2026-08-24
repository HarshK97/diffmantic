#include <iostream>
#include <string>
#include <string_view>
#include <vector>

namespace simdjson {
namespace ondemand {

struct error_code {
    int value = 0;
    explicit operator bool() const noexcept { return value != 0; }
};

struct raw_json_string {
    std::string_view raw;
    bool operator==(std::string_view s) const noexcept { return raw == s; }
};

struct field {
    raw_json_string key() const noexcept { return {}; }
    std::string_view value() const noexcept { return {}; }
};

struct object {
    std::vector<field> fields;
    auto begin() const noexcept { return fields.begin(); }
    auto end() const noexcept { return fields.end(); }
};

struct value {
    object get_object() const noexcept { return {}; }
};

struct Car {
    std::string make;
    std::string model;

    static auto deserialize(const value& val) -> Car {
        Car car{};
        auto obj = val.get_object();
        for (const auto& f : obj) {
            if (f.key() == "make") {
                car.make = f.value();
            } else if (f.key() == "model") {
                car.model = f.value();
            }
        }
        return car;
    }
};

} // namespace ondemand
} // namespace simdjson
