#pragma once

#include <cstdint>
#include <type_traits>

namespace Catch {
namespace Detail {

template <typename T>
struct ExtendedMultResult {
    T upper;
    T lower;

    friend bool operator==(ExtendedMultResult const& lhs,
                           ExtendedMultResult const& rhs) {
        return lhs.upper == rhs.upper && lhs.lower == rhs.lower;
    }

    friend bool operator!=(ExtendedMultResult const& lhs,
                           ExtendedMultResult const& rhs) {
        return !(lhs == rhs);
    }
};

template <typename UIntType>
constexpr ExtendedMultResult<UIntType> extendedMult(UIntType a, UIntType b) {
    uint64_t prod = static_cast<uint64_t>(a) * static_cast<uint64_t>(b);
    return {
        static_cast<UIntType>(prod >> 32),
        static_cast<UIntType>(prod)
    };
}

} // namespace Detail
} // namespace Catch
