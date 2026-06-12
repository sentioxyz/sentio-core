#!/usr/bin/env node
/*
 * protoc-gen-es-nodeprecated — filters descriptor elements before delegating
 * to protobuf-es.
 *
 * Strategy: stock @bufbuild/protoc-gen-es plus optional descriptor-rewrite
 * passes, each controlled by a plugin option set by the `es_proto` Bazel rule:
 *   remove_deprecated  strip `[deprecated=true]` elements
 *   strip_imports      drop options-only imports from dependency lists
 *   visibility_level   drop methods below a google.api visibility level
 * Each pass rewrites the CodeGeneratorRequest's FileDescriptorProtos before
 * delegating; with no options the request passes through unchanged. Output is
 * identical to stock protoc-gen-es, just minus the filtered elements — and they
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
import { BinaryReader, WireType } from '@bufbuild/protobuf/wire'
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

// --- filter methods by google.api visibility level ---------------------------
//
// Methods can be annotated with `(google.api.method_visibility).restriction`
// (google/api/visibility.proto). We use the labels as an ascending audience
// hierarchy: INTERNAL < PREVIEW < PUBLIC, where an unannotated method is PUBLIC.
// The `visibility_level` plugin option (set by the `es_proto` rule's attr of the
// same name) names the audience to generate for: a method is kept iff its level
// is >= the configured level. The default/absent level is INTERNAL (the lowest),
// which keeps everything — same as the openapi generator's behavior when no
// `visibility_restriction_selectors` are passed — and skips the pass entirely,
// so default builds stay byte-identical and never trip restriction validation.
//
// `restriction` is a comma-separated label list per google/api/visibility.proto
// ("INTERNAL,PREVIEW" = visible to both audiences); a multi-label method's level
// is its highest label. Unknown labels fail loudly — a typo must not silently
// add a method to (or hide it from) a public surface.
//
// Scope: METHODS only. field_/message_/enum_/value_/api_visibility are left
// untouched (filtering messages/enums could orphan type references; add field
// support alongside the deprecated-field oneof handling if ever needed). A
// service whose every method is filtered still emits an (empty) GenService.
//
// google/api/visibility.proto is normally in ES_STRIP_IMPORTS, so the extension
// is never registered — the option bytes sit in MethodOptions.$unknown, and we
// read them from there (extension field 72295727, a VisibilityRule message whose
// field 2 is `restriction`; field 1 `selector` is unused in option position).
const VISIBILITY_LEVELS = ['INTERNAL', 'PREVIEW', 'PUBLIC']
const VISIBILITY_EXT_FIELD = 72295727

// Returns the restriction string of a method, or undefined when unannotated.
function methodRestriction(method) {
  let restriction
  for (const f of method.options?.$unknown ?? []) {
    if (f.no !== VISIBILITY_EXT_FIELD || f.wireType !== WireType.LengthDelimited) continue
    // f.data is the length-prefixed VisibilityRule bytes. A repeated occurrence
    // of this singular extension merges per proto rules: last `restriction` wins.
    const rule = new BinaryReader(new BinaryReader(f.data).bytes())
    while (rule.pos < rule.len) {
      const [no, wt] = rule.tag()
      if (no === 2 && wt === WireType.LengthDelimited) restriction = rule.string()
      else rule.skip(wt, no)
    }
  }
  return restriction
}

// Maps a restriction label list to its level index; empty list = PUBLIC.
function visibilityLevelOf(restriction, where) {
  let level = -1
  for (const part of restriction.split(',')) {
    const label = part.trim()
    if (!label) continue
    const idx = VISIBILITY_LEVELS.indexOf(label)
    if (idx === -1) {
      throw new Error(
        `protoc-gen-es-nodeprecated: unknown google.api visibility restriction ` +
          `'${label}' on ${where}; known levels: ${VISIBILITY_LEVELS.join(' < ')}`
      )
    }
    if (idx > level) level = idx
  }
  return level === -1 ? VISIBILITY_LEVELS.indexOf('PUBLIC') : level
}

function stripHiddenMethods(req, minLevel) {
  const filterFile = (fd) => {
    const prefix = fd.package ? `${fd.package}.` : ''
    for (const svc of fd.service ?? []) {
      svc.method = (svc.method ?? []).filter((m) => {
        const restriction = methodRestriction(m)
        if (restriction === undefined) return true
        return visibilityLevelOf(restriction, `${prefix}${svc.name}.${m.name}`) >= minLevel
      })
    }
  }
  ;(req.protoFile ?? []).forEach(filterFile)
  ;(req.sourceFileDescriptors ?? []).forEach(filterFile)
  return req
}

// Consume the custom `visibility_level` option from the protoc parameter string
// (stock protoc-gen-es rejects unknown options). Returns the level index, or
// undefined when absent.
function takeVisibilityLevel(req) {
  const kept = []
  let level
  for (const part of (req.parameter ?? '').split(',')) {
    const opt = part.trim()
    if (!opt) continue
    const [key, value] = opt.split('=')
    if (key === 'visibility_level') {
      level = VISIBILITY_LEVELS.indexOf(value ?? '')
      if (level === -1) {
        throw new Error(
          `protoc-gen-es-nodeprecated: invalid visibility_level '${value}'; ` +
            `expected one of: ${VISIBILITY_LEVELS.join(', ')}`
        )
      }
    } else {
      kept.push(opt)
    }
  }
  req.parameter = kept.join(',')
  return level
}

runNodeJs({
  name: protocGenEs.name,
  version: protocGenEs.version,
  run: (req) => {
    if (takeRemoveDeprecated(req)) stripDeprecated(req)
    const toStrip = takeStripImports(req)
    if (toStrip.size > 0) stripImports(req, toStrip)
    const minVisibility = takeVisibilityLevel(req)
    if (minVisibility !== undefined && minVisibility > 0) stripHiddenMethods(req, minVisibility)
    return protocGenEs.run(req)
  }
})
