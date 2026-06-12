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

# protobuf-es (protoc-gen-es) TypeScript codegen. Always ESM
# (import_extension=js). `remove_deprecated = True` strips `[deprecated=true]`
# elements. No `grpc` attr is needed: protoc-gen-es always emits a GenService
# descriptor for every service (consumed directly by connect-es).
ES_BASE_OPTIONS = [
    "target=ts",
    "import_extension=js",
    "keep_empty_files=true",
]

# Imports that exist purely to declare custom-option extensions (annotations), never
# referenced as message/field TYPES. protobuf-es would otherwise emit a file-descriptor
# dependency + import for each (forcing those option protos to be generated too);
# the previous generator silently dropped all custom options, so it never did.
# The es-proto-plugin drops these
# from each FileDescriptorProto's `dependency` list (the option bytes remain as harmless
# unknown fields). Override per-target via the `strip_imports` attr.
ES_STRIP_IMPORTS = [
    "protoc-gen-openapiv2/options/annotations.proto",
    "google/api/annotations.proto",
    "google/api/field_behavior.proto",
    "google/api/visibility.proto",
    "google/api/client.proto",
]

# Ascending google.api visibility audience levels; an unannotated method is PUBLIC.
# Generating at a level keeps only methods at that level or above, so the default
# (INTERNAL, the lowest) keeps everything.
ES_VISIBILITY_LEVELS = ["INTERNAL", "PREVIEW", "PUBLIC"]

def es_proto_compile_impl(ctx):
    options = [] + ES_BASE_OPTIONS
    if ctx.attr.remove_deprecated:
        options += ["remove_deprecated=true"]
    if ctx.attr.strip_imports:
        options += ["strip_imports=" + ";".join(ctx.attr.strip_imports)]
    if ctx.attr.visibility_level:
        options += ["visibility_level=" + ctx.attr.visibility_level]
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
            doc = "Strip [deprecated=true] fields/messages/enums/services/methods from the generated code",
        ),
        strip_imports = attr.string_list(
            default = ES_STRIP_IMPORTS,
            doc = "Proto import paths to drop from the generated descriptor's dependency list " +
                  "(options-only annotation protos that protobuf-es would otherwise import). " +
                  "Defaults to the well-known annotation protos; set [] to disable.",
        ),
        visibility_level = attr.string(
            default = "",
            values = [""] + ES_VISIBILITY_LEVELS,
            doc = "Generate only methods visible at this google.api visibility level or above " +
                  "(per (google.api.method_visibility).restriction; unannotated methods are PUBLIC). " +
                  "Levels ascend INTERNAL < PREVIEW < PUBLIC. Defaults to the lowest level " +
                  "(INTERNAL), which keeps every method.",
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
