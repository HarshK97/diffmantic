#include <map>
#include <string>
#include <utility>
#include <vector>

namespace pybind11 {

struct object {
    std::string value;
    bool is_none_val = false;

    bool is_none() const noexcept {
        return is_none_val;
    }

    void write(const std::string& text) {
        // Output text
    }
};

void print(const std::vector<std::string>& args,
           const std::map<std::string, std::string>& kwargs,
           object& file) {
    std::string line;
    for (size_t i = 0; i < args.size(); ++i) {
        if (i > 0) {
            line += " ";
        }
        line += args[i];
    }

    file.write(std::move(line));
    auto it = kwargs.find("end");
    file.write(it != kwargs.end() ? it->second : "\n");
}

} // namespace pybind11
