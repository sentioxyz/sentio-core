"gateway_grpc_library wrapper macro"

load("@aspect_bazel_lib//lib:output_files.bzl", "make_output_files")
load("@aspect_bazel_lib//lib:write_source_files.bzl", "write_source_files")
load("@bazel_skylib//lib:paths.bzl", "paths")
load("@rules_proto_grpc_grpc_gateway//:defs.bzl", _gateway_grpc_compile = "gateway_grpc_compile")

def gateway_grpc_compile(name, proto_srcs = [], **kwargs):
    """Wrap gateway_grpc_compile with write_source_files and go_library.

    This causes the resulting .pb.go, _grpc.pb.go, and .pb.gw.go files
    to be checked into the source tree.

    Args:
        name: name of the final go_library rule produced
        proto_srcs: the srcs of the proto files
            If unset, a glob() of all ".proto" files in the package is used.
        **kwargs: remaining arguments to gateway_grpc_compile (like protos, deps, etc.)
    """

    # Based on your output, gateway_grpc_compile outputs to:
    # bazel-out/.../protos_gateway_compile/service/processor/protos/file.pb.go
    # The pattern is: {compile_name}/{package_name}/%s.pb.go

    package = native.package_name()
    compile_name = name + "_gateway_compile"

    # Output path pattern
    base_path = "{0}/{1}/{2}/%s".format(
        package,
        compile_name,
        package,
    )

    if len(proto_srcs) < 1:
        proto_srcs = native.glob(["*.proto"])

    # Generate all three file types
    _gateway_grpc_compile(
        name = compile_name,
        **kwargs
    )

    # Get base names without .proto extension
    bases = [paths.replace_extension(p, "") for p in proto_srcs]

    # Build the files dict by combining all three types
    files = {}

    # Add .pb.go files
    for base in bases:
        files[base + ".pb.go"] = make_output_files(
            base + "_pb_go",
            compile_name,
            [base_path % (base + ".pb.go")],
        )

    # Add _grpc.pb.go files
    for base in bases:
        files[base + "_grpc.pb.go"] = make_output_files(
            base + "_grpc_pb_go",
            compile_name,
            [base_path % (base + "_grpc.pb.go")],
        )

    # Add .pb.gw.go files
    for base in bases:
        files[base + ".pb.gw.go"] = make_output_files(
            base + "_gw_go",
            compile_name,
            [base_path % (base + ".pb.gw.go")],
        )

    # Write all generated files to source tree
    write_source_files(
        name = name + ".update_go_pb",
        files = files,
        visibility = ["//visibility:public"],
    )
