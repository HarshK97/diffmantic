#include <iostream>
#include <sstream>
#include <string>
#include <string_view>

namespace CLI {
namespace detail {

inline std::ostream& streamOutAsParagraph(std::ostream& out,
                                          std::string_view text,
                                          std::string_view linePrefix,
                                          std::size_t paragraphWidth,
                                          bool skipPrefixOnFirstLine) {
    std::istringstream lss(std::string(text));
    std::string line = "";
    while (std::getline(lss, line)) {
        std::istringstream iss(line);
        std::string word = "";
        std::size_t charsWritten = 0;

        if (!skipPrefixOnFirstLine) {
            out << linePrefix;
        }
        skipPrefixOnFirstLine = false;

        while (iss >> word) {
            if (charsWritten > 0 && (word.length() + 1 + charsWritten > paragraphWidth)) {
                out << '\n' << linePrefix;
                charsWritten = 0;
            }
            if (charsWritten > 0) {
                out << ' ';
                charsWritten++;
            }
            out << word;
            charsWritten += word.length();
        }
        out << '\n';
    }
    return out;
}

} // namespace detail
} // namespace CLI
