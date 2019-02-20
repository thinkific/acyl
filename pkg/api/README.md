# API Versioning

We now make the commitment to users that existing endpoints will not change,
either in schema/format or in content (for example, silently changing a branch name field to a SHA).

Any changed endpoints must be put into a separate API version.

This API package allows a developer to create a new API version by defining a new struct
in a new file in this package (`api_vX.go`) supporting the new routes it defines.

The API version struct is required to have a single method: `register(m *mux.Router)` which
accepts a Gorilla Mux router. It must define the various routes it implements along with their respective handler methods.

**DO NOT serialize or deserialize fundamental data structures in handlers** (anything in `models.go`).

Fundamental data types are subject to change at any time and in that event will break your API, often
silently.

Instead, define endpoint input/output structs as private types (ex: `apiV2_Environment`)
and then provide methods to convert to and from the fundamental types. If anyone changes
the fundamental types that break your API it will likely become a compile time error. Please create unit tests that verify endpoint conformance to your API contracts for better protection against silent breakage.

## HOWTO

1. Create a new files in this package (`api_v9.go`, `api_v9_test.go`).
2. Implement a non-exported struct and constructor. The constructor should take all required dependencies as parameters.
3. Implement the struct method `register()` and register all relevant endpoint routes using the Mux API.
4. Implement all handler methods.
5. Write tests
5. Wire in your structs creation and registration in `Dispatcher.RegisterVersions()`
6. Add a line to CHANGELOG describing the API changes and the new version number.
