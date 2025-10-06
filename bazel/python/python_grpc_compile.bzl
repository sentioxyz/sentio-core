"""
Custom Python gRPC compile rule that uses grpcio_tools.protoc directly.
This ensures Python builds use the precompiled protoc without affecting other languages.
"""

load("@rules_proto_grpc//:defs.bzl", "proto_compile_attrs", "proto_compile_toolchains")
load("@rules_proto//proto:defs.bzl", "ProtoInfo")

def _python_grpc_compile_impl(ctx):
    """Custom implementation that uses grpcio_tools protoc binary directly."""
    # Get the custom protoc binary
    protoc_executable = ctx.executable._grpcio_protoc
    
    # Get proto files from the proto_library
    proto_infos = [dep[ProtoInfo] for dep in ctx.attr.protos]
    
    # Collect all proto files
    proto_files = []
    for proto_info in proto_infos:
        proto_files.extend(proto_info.direct_sources)
    
    # Create output files for each proto file
    outputs = []
    for proto_file in proto_files:
        # Get the proto name without extension
        proto_name = proto_file.basename[:-6]  # Remove .proto extension
        
        # Create output files in the target directory (not package directory)
        pb2_file = ctx.actions.declare_file(proto_name + "_pb2.py")
        grpc_file = ctx.actions.declare_file(proto_name + "_pb2_grpc.py")
        outputs.extend([pb2_file, grpc_file])
    
    # Build protoc command
    protoc_args = ctx.actions.args()
    protoc_args.add("--python_out", ctx.bin_dir.path)
    protoc_args.add("--grpc_python_out", ctx.bin_dir.path)
    
    # Add include paths
    for proto_info in proto_infos:
        for path in proto_info.transitive_proto_path.to_list():
            protoc_args.add("-I", path)
    
    # Add proto files
    protoc_args.add_all(proto_files)
    
    # Run protoc
    ctx.actions.run(
        executable = protoc_executable,
        arguments = [protoc_args],
        inputs = proto_files + [dep for proto_info in proto_infos for dep in proto_info.transitive_sources.to_list()],
        outputs = outputs,
        mnemonic = "PythonGrpcCompile",
        progress_message = "Generating Python gRPC code for %{label}",
    )
    
    return [DefaultInfo(files = depset(outputs))]

# Create compile rule
python_grpc_compile = rule(
    implementation = _python_grpc_compile_impl,
    attrs = dict(
        proto_compile_attrs,
        _grpcio_protoc = attr.label(
            default = "//tools/grpcio_tools:protoc",
            executable = True,
            cfg = "exec",
            doc = "grpcio_tools protoc binary for Python gRPC compilation",
        ),
    ),
    toolchains = proto_compile_toolchains,
)
