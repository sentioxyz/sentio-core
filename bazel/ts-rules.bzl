load(
    "@rules_proto_grpc//:defs.bzl",
    "ProtoPluginInfo",
    "proto_compile_attrs",
    "proto_compile_impl",
    "proto_compile_toolchains",
)
load(
    "@rules_proto_grpc//internal:compile.bzl",
    "proto_compile",
)

BASE_OPTIONS = [
    "forceLong=bigint",
    "exportCommonSymbols=false",
    "esModuleInterop=true",
    "outputServices=generic-definitions",
    "useExactTypes=false",
]

GRPC_OPTIONS = [
    "outputServices=nice-grpc",
]

ESM_OPTIONS = ["importSuffix=.js"]

def ts_proto_compile_impl(ctx):
    options = [] + BASE_OPTIONS
    if ctx.attr.esm:
        options += ESM_OPTIONS
    if ctx.attr.grpc:
        options += GRPC_OPTIONS
    if ctx.attr.remove_deprecated:
        options += ["removeDeprecated=true"]
    options += ctx.attr.options

    extra_protoc_args = getattr(ctx.attr, "extra_protoc_args", [])
    extra_protoc_files = ctx.files.extra_protoc_files

    plugin_options = {
        "*": options,
    }

    return proto_compile(ctx, plugin_options, extra_protoc_args, extra_protoc_files)

ts_proto = rule(
    implementation = ts_proto_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        esm = attr.bool(
            default = False,
            doc = "Whether to generate ESM modules",
        ),
        grpc = attr.bool(
            default = False,
            doc = "Whether to generate gRPC services stub",
        ),
        options = attr.string_list(
            default = [],
            doc = "Additional options to pass to protoc",
        ),
        remove_deprecated = attr.bool(
            default = False,
            doc = "Whether to remove deprecated fields",
        ),
        _plugins = attr.label_list(
            providers = [ProtoPluginInfo],
            default = [
                Label("//bazel:ts-proto-plugin"),
            ],
            cfg = "exec",
            doc = "List of protoc plugins to apply",
        ),
    ),
    toolchains = proto_compile_toolchains,
)

ts_grpcgateway = rule(
    implementation = proto_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        _plugins = attr.label_list(
            providers = [ProtoPluginInfo],
            default = [
                Label("//bazel:ts-grpcgateway-plugin"),
            ],
            cfg = "exec",
            doc = "List of protoc plugins to apply",
        ),
    ),
    toolchains = proto_compile_toolchains,
)

# protobuf-es (protoc-gen-es) drop-in replacement for `ts_proto`. Always ESM
# (import_extension=js). `remove_deprecated = True` strips `[deprecated=true]`
# elements — the protobuf-es counterpart of ts_proto's removeDeprecated. No `grpc`
# attr is needed: protoc-gen-es always emits a GenService descriptor for every
# service (consumed directly by connect-es); there is no nice-grpc-style toggle.
ES_BASE_OPTIONS = [
    "target=ts",
    "import_extension=js",
    "keep_empty_files=true",
]

# Imports that exist purely to declare custom-option extensions (annotations), never
# referenced as message/field TYPES. protobuf-es would otherwise emit a file-descriptor
# dependency + import for each (forcing those option protos to be generated too); ts-proto
# silently dropped all custom options, so it never did. The es-proto-plugin drops these
# from each FileDescriptorProto's `dependency` list (the option bytes remain as harmless
# unknown fields). Override per-target via the `strip_imports` attr.
ES_STRIP_IMPORTS = [
    "protoc-gen-openapiv2/options/annotations.proto",
    "google/api/annotations.proto",
    "google/api/field_behavior.proto",
    "google/api/visibility.proto",
    "google/api/client.proto",
]

def es_proto_compile_impl(ctx):
    options = [] + ES_BASE_OPTIONS
    if ctx.attr.remove_deprecated:
        options += ["remove_deprecated=true"]
    if ctx.attr.strip_imports:
        options += ["strip_imports=" + ";".join(ctx.attr.strip_imports)]
    options += ctx.attr.options

    extra_protoc_args = getattr(ctx.attr, "extra_protoc_args", [])
    extra_protoc_files = ctx.files.extra_protoc_files

    return proto_compile(ctx, {"*": options}, extra_protoc_args, extra_protoc_files)

es_proto = rule(
    implementation = es_proto_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        remove_deprecated = attr.bool(
            default = False,
            doc = "Strip [deprecated=true] fields/messages/enums/services/methods from the generated code (protobuf-es counterpart of ts_proto's remove_deprecated)",
        ),
        strip_imports = attr.string_list(
            default = ES_STRIP_IMPORTS,
            doc = "Proto import paths to drop from the generated descriptor's dependency list " +
                  "(options-only annotation protos that protobuf-es would otherwise import). " +
                  "Defaults to the well-known annotation protos; set [] to disable.",
        ),
        options = attr.string_list(
            default = [],
            doc = "Additional options to pass to protoc-gen-es",
        ),
        _plugins = attr.label_list(
            providers = [ProtoPluginInfo],
            default = [
                Label("//bazel:es-proto-plugin"),
            ],
            cfg = "exec",
            doc = "List of protoc plugins to apply",
        ),
    ),
    toolchains = proto_compile_toolchains,
)
