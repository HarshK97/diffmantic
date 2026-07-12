# Test Fixtures

These are real-world bug-fix snippets we use for integration and end-to-end testing. They're extracted from open-source repositories.

## Structure

Each test case lives in its own folder containing:

- `old.<ext>` — The buggy version of the file
- `new.<ext>` — The fixed version
- `metadata.json` — Git metadata (repo, commit, author, changes count)
- `expected.json` — The golden diff output

## Test Cases (15 per language)

### Go

| Fixture                                                 | Source        | Description                                                                 |
| ------------------------------------------------------- | ------------- | --------------------------------------------------------------------------- |
| `go_binding_validation`                                 | gin-gonic/gin | fix(binding): prevent duplicate decoding and add validation in decodeTom... |
| `go_comment_update`                                     | gin-gonic/gin | docs: fix `BindXML` comment referencing nonexistent `binding.BindXML` (#... |
| `go_cookie_reset_fix`                                   | gin-gonic/gin | Fix: missing `sameSite` when do context.reset() (#3123)                     |
| `go_copy_copies_errors`                                 | gin-gonic/gin | fix(context): Copy() copies Errors and Accepted fields (#4695)              |
| `go_fix_bindxml_comment`                                | gin-gonic/gin | docs: fix `BindXML` comment referencing nonexistent `binding.BindXML` (#... |
| `go_format_string_fix`                                  | gin-gonic/gin | Fix not to pass formatted string to Fprintf's format specifier parameter... |
| `go_ipv6_default`                                       | gin-gonic/gin | fix: the trusted proxies should support ipv6 address by default (#3033)     |
| `go_map_copy_guard`                                     | gin-gonic/gin | fix: protect Context.Keys map when call Copy method (#3873)                 |
| `go_move_guard`                                         | gin-gonic/gin | fix(binding): dereference pointer to struct (#3199)                         |
| `go_panic_findcaseinsensitivepathrec_redirectfixedpath` | gin-gonic/gin | fix(tree): panic in findCaseInsensitivePathRec with RedirectFixedPath (#... |
| `go_postform_cache`                                     | gin-gonic/gin | Attempt to fix PostForm cache bug (#1931)                                   |
| `go_record_recovered_panics`                            | gin-gonic/gin | fix(recovery): record recovered panics in c.Errors (#4698)                  |
| `go_resource_leak_fix`                                  | gin-gonic/gin | fix(gin): close os.File in RunFd to prevent resource leak (#4422)           |
| `go_version_mismatch`                                   | gin-gonic/gin | fix(debug): version mismatch (#4403)                                        |
| `go_write_content_length`                               | gin-gonic/gin | fix(render): write content length in Data.Render (#4206)                    |

### JavaScript

| Fixture                           | Source            | Description                                                                 |
| --------------------------------- | ----------------- | --------------------------------------------------------------------------- |
| `js_add_content_length`           | expressjs/express | fix(res.send): add Content-Length header only if Transfer-Encoding is no... |
| `js_allow_passing_null`           | expressjs/express | feat: Allow passing null or undefined as the value for options in app.re... |
| `js_append_typo_fix`              | expressjs/express | chore: fix typo (#6609)                                                     |
| `js_assertion_fix`                | expressjs/express | tests: fix test missing assertion                                           |
| `js_boot_filter_fix`              | expressjs/express | examples: fix mvc example to ignore files in controllers dir                |
| `js_callback_fix`                 | expressjs/express | tests: fix callback in res.download test                                    |
| `js_jsdoc_fix`                    | expressjs/express | docs: fix incomplete JSDoc comment                                          |
| `js_jsonp_deprecation`            | expressjs/express | Fix res.jsonp(obj, status) deprecation message                              |
| `js_move_detection`               | expressjs/express | fix(examples): improve readability of user assignment (#6190)               |
| `js_prefer_referer_header`        | expressjs/express | fix: prefer Referer header over Referrer                                    |
| `js_prototype_fix`                | expressjs/express | Fix constructing application with non-configurable prototype properties     |
| `js_replace_deprecated_trimright` | expressjs/express | fix: replace deprecated trimRight() with trimEnd() (#7265)                  |
| `js_response_doc_fix`             | expressjs/express | docs: fix typo in private api jsdoc                                         |
| `js_restore_array_parsing`        | expressjs/express | fix: restore array parsing for req.query repeated keys (#7181)              |
| `js_token_update`                 | expressjs/express | fix: replace deprecated trimRight() with trimEnd() (#7265)                  |

### Python

| Fixture                     | Source       | Description                                                                 |
| --------------------------- | ------------ | --------------------------------------------------------------------------- |
| `py_appengine_fix`          | psf/requests | Fix for AppEngine                                                           |
| `py_auth_redirect`          | psf/requests | A cleaner and more complete fix for auth/redirects                          |
| `py_cookie_redirect`        | psf/requests | Fix Setting a cookie on redirect                                            |
| `py_delimiter_fix`          | psf/requests | Fix bug when delimiter is split between responses                           |
| `py_digest_auth_uri`        | psf/requests | Bug fix: field uri in digest authentication should not be empty when enc... |
| `py_fix_cosmetic_header`    | psf/requests | Fix cosmetic header validity parsing regex (#7308)                          |
| `py_fix_empty_netrc`        | psf/requests | Fix empty netrc entry usage (#7205)                                         |
| `py_fix_encode_files`       | psf/requests | Fix `_encode_files` detection for `__getattr__`-based file wrappers (#7502) |
| `py_fix_prepare_body`       | psf/requests | Fix `prepare_body` stream detection for `__getattr__`-based file wrapper... |
| `py_fix_remaining_typos`    | psf/requests | Fix remaining typos (#7395)                                                 |
| `py_fix_typo_documentation` | psf/requests | Fix typo in documentation for verify                                        |
| `py_guard_insert`           | psf/requests | Fix the proxy_bypass_registry function all returning true in some cases.    |
| `py_python3_compat`         | psf/requests | Fixes python3 compatibility issue                                           |
| `py_str_encoding_len`       | psf/requests | Enhance `super_len` to count encoded bytes for str                          |
| `py_utf8_bom_fix`           | psf/requests | fix response with utf8 bom                                                  |

### TypeScript

| Fixture                                              | Source          | Description                                                                   |
| ---------------------------------------------------- | --------------- | ----------------------------------------------------------------------------- |
| `ts_circular_import`                                 | typeorm/typeorm | fix: circular import in SapDriver.ts (#11750)                                 |
| `ts_cli_bug_fix`                                     | typeorm/typeorm | fix: bug introduced in CLI (#9332)                                            |
| `ts_correct_grammar_alreadyhasactiveconnectionerror` | typeorm/typeorm | fix: correct grammar in AlreadyHasActiveConnectionError message (#12554)      |
| `ts_error_message_fix`                               | typeorm/typeorm | fix: correct grammar in AlreadyHasActiveConnectionError message (#12554)      |
| `ts_guard_insert`                                    | typeorm/typeorm | fix(query-builder): only normalize plain-object where criteria, guard \_\_... |
| `ts_isolation_level`                                 | typeorm/typeorm | fix(mssql): throw on unknown isolation level instead of silent fallback       |
| `ts_missing_export`                                  | typeorm/typeorm | fix: add missing export for View class (#10261)                               |
| `ts_mongo_compat`                                    | typeorm/typeorm | fix: mongodb@4 compatibility support (#8412)                                  |
| `ts_only_normalize_plain`                            | typeorm/typeorm | fix(query-builder): only normalize plain-object where criteria, guard \_\_... |
| `ts_preserve_select_false`                           | typeorm/typeorm | fix(persistence): preserve select false columns on the in-memory entity ...   |
| `ts_preserve_select_false_1`                         | typeorm/typeorm | fix(persistence): preserve select false columns on the in-memory entity ...   |
| `ts_prototype_guard`                                 | typeorm/typeorm | fix: prototype pollution issue                                                |
| `ts_query_result_fix`                                | typeorm/typeorm | fix: copy cordova query rows affected into query result (#10873)              |
| `ts_release_query_runner`                            | typeorm/typeorm | fix(cache): release query runner on error in storeInCache (#12545)            |
| `ts_subtree_delete`                                  | typeorm/typeorm | fix(persistence): preserve select false columns on the in-memory entity ...   |

## Licensing

These snippets come from open-source projects under the following licenses:

- **gin-gonic/gin**: MIT
- **expressjs/express**: MIT
- **psf/requests**: Apache-2.0
- **typeorm/typeorm**: MIT

Check the `metadata.json` inside each folder for author attributions and exact commit hashes.

## Updating Golden Files

If you make engine changes that affect the diff outputs, regenerate the golden files by running:

```bash
go test ./tests/integration/ -v -update
```
