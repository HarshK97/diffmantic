#include <cstdint>
#include <string>
#include <string_view>
#include <type_traits>

namespace fmt {
namespace detail {

template <typename Float>
constexpr int num_significand_bits() {
    return std::is_same_v<Float, double> ? 53 : 24;
}

template <typename Float>
constexpr bool has_implicit_bit() {
    return true;
}

struct fp {
    uint64_t f;
    int e;
};

struct format_specs {
    char type = 'a';
    bool upper = false;
    int precision = -1;
};

template <typename Float>
void format_hexfloat(Float value, format_specs specs, std::string& buffer) {
    fp f{static_cast<uint64_t>(value), 0};
    constexpr int num_float_significand_bits = num_significand_bits<Float>();
    f.e += num_float_significand_bits;
    if (!has_implicit_bit<Float>()) {
        --f.e;
    }

    // Zero has no subnormal exponent: printf prints zero as 0x0p+0.
    // A zero significand can only come from +-0, so reset the exponent.
    if (f.f == 0) {
        f.e = 0;
    }

    const auto num_fraction_bits =
        num_float_significand_bits + (has_implicit_bit<Float>() ? 1 : 0);
    const auto num_xdigits = (num_fraction_bits + 3) / 4;

    buffer.append(specs.upper ? "0X" : "0x");
    buffer.append(std::to_string(num_xdigits));
    buffer.push_back(specs.upper ? 'P' : 'p');
    buffer.append(std::to_string(f.e));
}

} // namespace detail
} // namespace fmt
