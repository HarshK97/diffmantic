#include "bridge.h"
#include <tree_sitter/api.h>
#include <string.h>

extern const TSLanguage* tree_sitter_c(void);
extern const TSLanguage* tree_sitter_cpp(void);
extern const TSLanguage* tree_sitter_go(void);
extern const TSLanguage* tree_sitter_rust(void);
extern const TSLanguage* tree_sitter_python(void);
extern const TSLanguage* tree_sitter_javascript(void);
extern const TSLanguage* tree_sitter_typescript(void);
extern const TSLanguage* tree_sitter_tsx(void);
extern const TSLanguage* tree_sitter_java(void);
extern const TSLanguage* tree_sitter_lua(void);
extern const TSLanguage* tree_sitter_zig(void);

const TSLanguage* diffmantic_get_native_language(const char* name) {
    if (!name) return NULL;
    if (strcmp(name, "c") == 0) return tree_sitter_c();
    if (strcmp(name, "cpp") == 0) return tree_sitter_cpp();
    if (strcmp(name, "go") == 0) return tree_sitter_go();
    if (strcmp(name, "rust") == 0 || strcmp(name, "rs") == 0) return tree_sitter_rust();
    if (strcmp(name, "python") == 0 || strcmp(name, "py") == 0) return tree_sitter_python();
    if (strcmp(name, "javascript") == 0 || strcmp(name, "js") == 0) return tree_sitter_javascript();
    if (strcmp(name, "typescript") == 0 || strcmp(name, "ts") == 0) return tree_sitter_typescript();
    if (strcmp(name, "tsx") == 0) return tree_sitter_tsx();
    if (strcmp(name, "java") == 0) return tree_sitter_java();
    if (strcmp(name, "lua") == 0) return tree_sitter_lua();
    if (strcmp(name, "zig") == 0) return tree_sitter_zig();
    return NULL;
}
