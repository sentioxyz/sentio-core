#!/usr/bin/env node
/*
 * protoc-gen-es-nodeprecated — removes deprecated descriptor elements before
 * delegating to protobuf-es.
 *
 * Strategy: stock @bufbuild/protoc-gen-es plus an optional `remove_deprecated`
 * pass, controlled by a plugin option of the same name (set by the `es_proto`
 * Bazel rule's `remove_deprecated` attr). When set, strip `[deprecated=true]`
 * elements from the CodeGeneratorRequest's FileDescriptorProtos before
 * delegating; otherwise pass through unchanged. Output is identical to stock
 * protoc-gen-es, just minus the deprecated elements when requested — and they
 * also disappear from the embedded base64 descriptor, so the user-facing copy
 * stays wire-compatible with the runtime copy that keeps them (the field is
 * simply an unknown field for this copy).
 *
 * Scope (matches what processor.proto uses + the safe subset):
 *   removed: deprecated fields, whole messages, whole enums, services, methods, extensions
 *   KEPT:    individual enum VALUES (avoids enum-value type-conflict issues)
 *            oneof declarations (kept so surviving fields' oneofIndex stays valid)
 *
 * NOT implemented (fails loudly — see assertNoOrphanedOneof): removing a field that
 * empties a oneof. The two ways that happens are (a) a deprecated proto3 `optional`
 * field, whose synthetic oneof `_field` would be orphaned, and (b) a oneof whose
 * every member is deprecated. Either needs pruning the oneof_decl AND reindexing the
 * oneof_index of surviving fields. processor.proto has neither today, so this plugin
 * throws instead of emitting a malformed descriptor. Implement reindexing if needed.
 */
import { runNodeJs } from '@bufbuild/protoplugin'
// @bufbuild/protoc-gen-es ships CJS only and does not expose protocGenEs on its
// package export map; this is the exact module its own bin/protoc-gen-es loads.
// Node's ESM->CJS interop resolves the named export. Pinned by the lockfile.
import { protocGenEs } from '@bufbuild/protoc-gen-es/dist/cjs/src/protoc-gen-es-plugin.js'

const isDeprecated = (d) => !!(d && d.options && d.options.deprecated === true)

// Fail loudly if dropping deprecated fields left a oneof with no members.
function assertNoOrphanedOneof(msg, fqName) {
  const referenced = new Set()
  for (const f of msg.field ?? []) {
    if (f.oneofIndex !== undefined) referenced.add(f.oneofIndex)
  }
  ;(msg.oneofDecl ?? []).forEach((o, i) => {
    if (!referenced.has(i)) {
      const kind =
        o.name && o.name.startsWith('_')
          ? 'synthetic oneof of a proto3 `optional` field'
          : 'oneof'
      throw new Error(
        `protoc-gen-es-nodeprecated: dropping deprecated fields emptied the ${kind} ` +
          `'${o.name}' in message '${fqName}'. Pruning + reindexing oneofs is not ` +
          `implemented (this plugin is scoped to processor.proto). Implement it before ` +
          `marking a proto3 'optional' field — or every member of a oneof — deprecated.`
      )
    }
  })
}

function filterMessage(msg, fqName) {
  msg.field = (msg.field ?? []).filter((f) => !isDeprecated(f))
  assertNoOrphanedOneof(msg, fqName)
  msg.nestedType = (msg.nestedType ?? []).filter((m) => !isDeprecated(m))
  msg.nestedType.forEach((m) => filterMessage(m, `${fqName}.${m.name}`))
  msg.enumType = (msg.enumType ?? []).filter((e) => !isDeprecated(e)) // keep enum VALUES
  msg.extension = (msg.extension ?? []).filter((x) => !isDeprecated(x))
}

function filterFile(fd) {
  const prefix = fd.package ? `${fd.package}.` : ''
  fd.messageType = (fd.messageType ?? []).filter((m) => !isDeprecated(m))
  fd.messageType.forEach((m) => filterMessage(m, `${prefix}${m.name}`))
  fd.enumType = (fd.enumType ?? []).filter((e) => !isDeprecated(e)) // keep enum VALUES
  fd.extension = (fd.extension ?? []).filter((x) => !isDeprecated(x))
  fd.service = (fd.service ?? []).filter((s) => !isDeprecated(s))
  fd.service.forEach((s) => {
    s.method = (s.method ?? []).filter((m) => !isDeprecated(m))
  })
}

function stripDeprecated(req) {
  ;(req.protoFile ?? []).forEach(filterFile)
  ;(req.sourceFileDescriptors ?? []).forEach(filterFile)
  return req
}

// Consume our custom `remove_deprecated` option from the protoc parameter string
// and strip it out — stock protoc-gen-es rejects unknown options, so it must never
// see it. Returns whether deprecated elements should be removed.
function takeRemoveDeprecated(req) {
  const kept = []
  let remove = false
  for (const part of (req.parameter ?? '').split(',')) {
    const opt = part.trim()
    if (!opt) continue
    const [key, value] = opt.split('=')
    if (key === 'remove_deprecated') {
      remove = value === undefined || value === 'true' || value === '1'
    } else {
      kept.push(opt)
    }
  }
  req.parameter = kept.join(',')
  return remove
}

// --- strip options-only imports -------------------------------------------
//
// Some protos import files purely for their custom-option extensions (e.g.
// grpc-gateway's openapiv2 annotations, google.api.http/field_behavior/visibility).
// protobuf-es is descriptor-faithful, so it emits a file-descriptor dependency +
// import for every imported .proto — which would force generating those option
  // protos too (legacy codegen silently dropped all custom options, so it never did).
//
// Since NO message/field references a TYPE from these option-only files (they appear
// only in options), we can drop them from each FileDescriptorProto's `dependency`
// list before delegating. protobuf-es then omits the import + the dep from the
// generated `fileDesc(b64, [deps])`; the option bytes remain as harmless unknown
// fields in the embedded descriptor. Controlled by the `strip_imports` plugin option
// (a `;`-separated list of proto import paths), set by the `es_proto` rule.
function stripImportsFromFile(fd, toStrip) {
  const deps = fd.dependency ?? []
  if (deps.length === 0) return
  const removed = new Set()
  deps.forEach((dep, i) => {
    if (toStrip.has(dep)) removed.add(i)
  })
  if (removed.size === 0) return
  // Remap surviving dependency indices (public/weak dependency lists hold indices).
  const remap = new Map()
  let next = 0
  deps.forEach((_, i) => {
    if (!removed.has(i)) remap.set(i, next++)
  })
  fd.dependency = deps.filter((_, i) => !removed.has(i))
  if (fd.publicDependency) {
    fd.publicDependency = fd.publicDependency.filter((i) => !removed.has(i)).map((i) => remap.get(i))
  }
  if (fd.weakDependency) {
    fd.weakDependency = fd.weakDependency.filter((i) => !removed.has(i)).map((i) => remap.get(i))
  }
}

function stripImports(req, toStrip) {
  ;(req.protoFile ?? []).forEach((fd) => stripImportsFromFile(fd, toStrip))
  ;(req.sourceFileDescriptors ?? []).forEach((fd) => stripImportsFromFile(fd, toStrip))
  return req
}

// Consume the custom `strip_imports` option (a `;`-separated list of import paths)
// from the protoc parameter string and strip it out before stock protoc-gen-es
// (which rejects unknown options) sees it. Returns the Set of import paths to drop.
function takeStripImports(req) {
  const kept = []
  let toStrip = new Set()
  for (const part of (req.parameter ?? '').split(',')) {
    const opt = part.trim()
    if (!opt) continue
    const eq = opt.indexOf('=')
    const key = eq === -1 ? opt : opt.slice(0, eq)
    const value = eq === -1 ? undefined : opt.slice(eq + 1)
    if (key === 'strip_imports') {
      toStrip = new Set(
        (value ?? '')
          .split(';')
          .map((s) => s.trim())
          .filter(Boolean)
      )
    } else {
      kept.push(opt)
    }
  }
  req.parameter = kept.join(',')
  return toStrip
}

runNodeJs({
  name: protocGenEs.name,
  version: protocGenEs.version,
  run: (req) => {
    if (takeRemoveDeprecated(req)) stripDeprecated(req)
    const toStrip = takeStripImports(req)
    if (toStrip.size > 0) stripImports(req, toStrip)
    return protocGenEs.run(req)
  }
})
