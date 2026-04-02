#!/usr/bin/env python3
"""
Verify a Sui gRPC endpoint is reachable and healthy.

Usage:
    python3 scripts/check_sui_grpc.py <endpoint> [--subscribe-secs N]

Examples:
    python3 scripts/check_sui_grpc.py https://sui-grpc.example.com
    python3 scripts/check_sui_grpc.py http://localhost:50051
    python3 scripts/check_sui_grpc.py https://sui-grpc.example.com --subscribe-secs 5

The script:
  1. Dials the endpoint (http:// → plaintext, https:// → TLS).
  2. Calls LedgerService.GetServiceInfo and prints the result.
  3. Optionally subscribes to SubscriptionService.SubscribeCheckpoints for
     --subscribe-secs seconds and prints each received checkpoint.

Dependencies (install once):
    pip install grpcio grpcio-tools googleapis-common-protos
"""

import argparse
import importlib
import os
import sys
import tempfile
import time

import grpc
from google.protobuf import field_mask_pb2


# ---------------------------------------------------------------------------
# Both services are compiled together from a single proto file so that
# Python's module cache never serves stale stubs.
# ---------------------------------------------------------------------------

COMBINED_PROTO = """
syntax = "proto3";
package sui.rpc.v2;
import "google/protobuf/field_mask.proto";
import "google/protobuf/timestamp.proto";

// ── LedgerService ────────────────────────────────────────────────────────────

message GetServiceInfoRequest {}

message GetServiceInfoResponse {
  optional string chain_id          = 1;
  optional string chain             = 2;
  optional uint64 epoch             = 3;
  optional uint64 checkpoint_height = 4;
  optional google.protobuf.Timestamp timestamp = 5;
  optional uint64 lowest_available_checkpoint = 6;
  optional uint64 lowest_available_checkpoint_objects = 7;
  optional string server            = 8;
}

service LedgerService {
  rpc GetServiceInfo(GetServiceInfoRequest) returns (GetServiceInfoResponse);
}

// ── SubscriptionService ───────────────────────────────────────────────────────

message CheckpointSummary {
  optional google.protobuf.Timestamp timestamp = 9;
}

message Checkpoint {
  optional uint64            sequence_number = 1;
  optional string            digest          = 2;
  optional CheckpointSummary summary         = 3;
}

message SubscribeCheckpointsRequest {
  optional google.protobuf.FieldMask read_mask = 1;
}

message SubscribeCheckpointsResponse {
  optional uint64     cursor     = 1;
  optional Checkpoint checkpoint = 2;
}

service SubscriptionService {
  rpc SubscribeCheckpoints(SubscribeCheckpointsRequest)
      returns (stream SubscribeCheckpointsResponse);
}
"""

# Compiled once at startup; populated by _ensure_compiled().
_pb2 = None
_pb2_grpc = None


def _ensure_compiled():
    """Compile COMBINED_PROTO once and cache the resulting modules."""
    global _pb2, _pb2_grpc
    if _pb2 is not None:
        return

    try:
        from grpc_tools import protoc
    except ImportError:
        print("ERROR: grpc_tools not found.  Install with:  pip install grpcio-tools")
        sys.exit(1)

    import grpc_tools
    grpc_tools_path = os.path.dirname(grpc_tools.__file__)
    include_path = os.path.join(grpc_tools_path, "_proto")

    tmpdir = tempfile.mkdtemp(prefix="sui_grpc_check_")
    proto_path = os.path.join(tmpdir, "sui_rpc.proto")
    out_dir = os.path.join(tmpdir, "out")
    os.makedirs(out_dir)

    with open(proto_path, "w") as f:
        f.write(COMBINED_PROTO)

    ret = protoc.main([
        "grpc_tools.protoc",
        f"--proto_path={tmpdir}",
        f"--proto_path={include_path}",
        f"--python_out={out_dir}",
        f"--grpc_python_out={out_dir}",
        proto_path,
    ])
    if ret != 0:
        print("ERROR: protoc compilation failed.")
        sys.exit(1)

    sys.path.insert(0, out_dir)
    _pb2      = importlib.import_module("sui_rpc_pb2")
    _pb2_grpc = importlib.import_module("sui_rpc_pb2_grpc")
    sys.path.pop(0)


def _make_channel(endpoint: str) -> grpc.Channel:
    """Create a gRPC channel; http:// → insecure, https:// → TLS."""
    if endpoint.startswith("http://"):
        target = endpoint[len("http://"):]
        return grpc.insecure_channel(target)
    else:
        target = endpoint.removeprefix("https://")
        creds = grpc.ssl_channel_credentials()
        return grpc.secure_channel(target, creds)


def check_service_info(channel: grpc.Channel) -> bool:
    """Call GetServiceInfo and print the result.  Returns True on success."""
    _ensure_compiled()
    stub = _pb2_grpc.LedgerServiceStub(channel)
    try:
        resp = stub.GetServiceInfo(_pb2.GetServiceInfoRequest(), timeout=5)
    except grpc.RpcError as e:
        print(f"  [FAIL] GetServiceInfo: {e.code()} — {e.details()}")
        return False

    ts = resp.timestamp
    ts_str = f"{ts.seconds}" if ts else "N/A"

    print(f"  [OK] GetServiceInfo")
    print(f"       chain_id          : {resp.chain_id or 'N/A'}")
    print(f"       chain             : {resp.chain or 'N/A'}")
    print(f"       epoch             : {resp.epoch}")
    print(f"       checkpoint_height : {resp.checkpoint_height}")
    print(f"       timestamp (unix)  : {ts_str}")
    print(f"       server            : {resp.server or 'N/A'}")
    return True


def check_subscribe(channel: grpc.Channel, secs: int) -> bool:
    """Subscribe to checkpoints for `secs` seconds.  Returns True if at least one received."""
    _ensure_compiled()
    stub = _pb2_grpc.SubscriptionServiceStub(channel)

    req = _pb2.SubscribeCheckpointsRequest()
    # Request minimal fields: sequence_number, digest, summary.timestamp
    req.read_mask.CopyFrom(
        field_mask_pb2.FieldMask(paths=["sequence_number", "digest", "summary.timestamp"])
    )

    print(f"\n  Subscribing to checkpoints for {secs}s ...")
    received = 0
    deadline = time.time() + secs
    try:
        for resp in stub.SubscribeCheckpoints(req, timeout=secs + 2):
            cp = resp.checkpoint
            ts = cp.summary.timestamp
            ts_str = f"{ts.seconds}" if ts.seconds else "N/A"
            print(f"  [checkpoint] seq={cp.sequence_number}  digest={cp.digest}  ts={ts_str}")
            received += 1
            if time.time() >= deadline:
                break
    except grpc.RpcError as e:
        if e.code() == grpc.StatusCode.DEADLINE_EXCEEDED and received > 0:
            pass  # expected: we hit our timeout after receiving some data
        else:
            print(f"  [FAIL] SubscribeCheckpoints: {e.code()} — {e.details()}")
            return False

    if received == 0:
        print("  [WARN] No checkpoints received within the timeout.")
        return False

    print(f"  [OK] Received {received} checkpoint(s).")
    return True


def main():
    parser = argparse.ArgumentParser(description="Verify a Sui gRPC endpoint.")
    parser.add_argument("endpoint", help="Endpoint URL, e.g. https://sui-grpc.example.com or http://localhost:50051")
    parser.add_argument("--subscribe-secs", type=int, default=0, metavar="N",
                        help="Also subscribe to checkpoints for N seconds (default: 0 = skip)")
    args = parser.parse_args()

    endpoint = args.endpoint
    print(f"Checking Sui gRPC endpoint: {endpoint}\n")

    channel = _make_channel(endpoint)

    ok = check_service_info(channel)
    if not ok:
        sys.exit(1)

    if args.subscribe_secs > 0:
        ok = check_subscribe(channel, args.subscribe_secs)
        if not ok:
            sys.exit(1)

    print("\nEndpoint looks healthy.")


if __name__ == "__main__":
    main()
