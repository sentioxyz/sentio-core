/*
 * Unit tests for visibility-surface.mjs. Descriptors are plain objects in the
 * protobuf-es decoded shape (camelCase fields, options.$unknown for custom
 * option bytes) — exactly what the pass sees inside the plugin. The golden
 * end-to-end tests against real protoc live in //bazel/protoc-gen-es/tests.
 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { BinaryWriter, WireType } from '@bufbuild/protobuf/wire'
import {
  VISIBILITY_LEVELS,
  applyVisibilitySurface,
  restrictionOf,
  scrubComment
} from './visibility-surface.mjs'

const PREVIEW = VISIBILITY_LEVELS.indexOf('PREVIEW')
const PUBLIC = VISIBILITY_LEVELS.indexOf('PUBLIC')

const VISIBILITY_EXT = 72295727
const HTTP_EXT = 72295728

// (google.api.*_visibility) option bytes: a VisibilityRule whose field 2 is
// `restriction`, length-prefixed the way protobuf-es stores unknown fields.
function vis(restriction) {
  const rule = new BinaryWriter()
    .tag(2, WireType.LengthDelimited)
    .string(restriction)
    .finish()
  return {
    no: VISIBILITY_EXT,
    wireType: WireType.LengthDelimited,
    data: new BinaryWriter().bytes(rule).finish()
  }
}

function httpRule() {
  const rule = new BinaryWriter()
    .tag(2, WireType.LengthDelimited)
    .string('/v1/things')
    .finish()
  return {
    no: HTTP_EXT,
    wireType: WireType.LengthDelimited,
    data: new BinaryWriter().bytes(rule).finish()
  }
}

const str = (name, number, extra = {}) => ({
  name,
  number,
  label: 1,
  type: 9,
  ...extra
})
const msgField = (name, number, typeName, extra = {}) => ({
  name,
  number,
  label: 1,
  type: 11,
  typeName,
  ...extra
})
const enumField = (name, number, typeName, extra = {}) => ({
  name,
  number,
  label: 1,
  type: 14,
  typeName,
  ...extra
})

const method = (name, inputType, outputType, extra = {}) => ({
  name,
  inputType,
  outputType,
  ...extra
})

// A representative two-file request:
//   svc.proto (estest): PublicApi {Get (public, http), Peek (preview), Admin (internal)},
//                       HiddenApi (api_visibility INTERNAL), DeadApi (all methods internal)
//   types: Req (internal field, oneof w/ internal member, internal optional,
//          map<string,dep.Shared>, enum Kind w/ internal value, nested Used/Unused),
//          Resp (self-recursive), AdminReq -> Detail (internal-only closure)
//   dep.proto (estest.dep): Shared (kept via map), Orphan (pruned)
function makeReq() {
  const svcFile = {
    name: 'svc.proto',
    package: 'estest',
    dependency: ['dep.proto', 'google/api/visibility.proto'],
    messageType: [
      {
        name: 'Req',
        field: [
          str('id', 1),
          str('debug', 2, { options: { $unknown: [vis('INTERNAL')] } }),
          str('a', 3, { oneofIndex: 0 }),
          str('b', 4, {
            oneofIndex: 0,
            options: { $unknown: [vis('INTERNAL')] }
          }),
          str('gone', 5, {
            proto3Optional: true,
            oneofIndex: 1,
            options: { $unknown: [vis('INTERNAL')] }
          }),
          str('kept', 6, { proto3Optional: true, oneofIndex: 2 }),
          msgField('shared', 7, '.estest.Req.SharedEntry', { label: 3 }),
          enumField('kind', 8, '.estest.Kind'),
          msgField('used', 9, '.estest.Req.Used')
        ],
        oneofDecl: [{ name: 'sel' }, { name: '_gone' }, { name: '_kept' }],
        nestedType: [
          {
            name: 'SharedEntry',
            field: [str('key', 1), msgField('value', 2, '.estest.dep.Shared')],
            options: { mapEntry: true },
            nestedType: [],
            enumType: [],
            oneofDecl: [],
            extension: []
          },
          {
            name: 'Used',
            field: [str('x', 1)],
            nestedType: [],
            enumType: [],
            oneofDecl: [],
            extension: []
          },
          {
            name: 'Unused',
            field: [str('y', 1)],
            nestedType: [],
            enumType: [],
            oneofDecl: [],
            extension: []
          }
        ],
        enumType: [],
        extension: []
      },
      {
        name: 'Resp',
        field: [msgField('next', 1, '.estest.Resp')],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      },
      {
        name: 'AdminReq',
        field: [msgField('detail', 1, '.estest.Detail')],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      },
      {
        name: 'Detail',
        field: [str('secret', 1)],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      }
    ],
    enumType: [
      {
        name: 'Kind',
        value: [
          { name: 'KIND_UNSPECIFIED', number: 0 },
          { name: 'KIND_A', number: 1 },
          {
            name: 'KIND_SECRET',
            number: 2,
            options: { $unknown: [vis('INTERNAL')] }
          }
        ]
      },
      { name: 'DeadEnum', value: [{ name: 'DEAD', number: 0 }] }
    ],
    service: [
      {
        name: 'PublicApi',
        method: [
          method('Get', '.estest.Req', '.estest.Resp', {
            options: { $unknown: [httpRule(), vis('PUBLIC')] }
          }),
          method('Peek', '.estest.Req', '.estest.Resp', {
            options: { $unknown: [vis('PREVIEW')] }
          }),
          method('Admin', '.estest.AdminReq', '.estest.Resp', {
            options: { $unknown: [vis('INTERNAL')] }
          })
        ]
      },
      {
        name: 'HiddenApi',
        options: { $unknown: [vis('INTERNAL')] },
        method: [method('X', '.estest.Req', '.estest.Resp')]
      },
      {
        name: 'DeadApi',
        method: [
          method('Y', '.estest.Req', '.estest.Resp', {
            options: { $unknown: [vis('INTERNAL')] }
          })
        ]
      }
    ],
    extension: [
      str('my_ext', 50000, { extendee: '.google.protobuf.MethodOptions' })
    ],
    options: { $unknown: [vis('PUBLIC')], javaPackage: 'com.estest' },
    sourceCodeInfo: {
      location: [
        {
          path: [4, 0],
          leadingComments: ' Request. (-- internal note --) Keep this.\n'
        },
        { path: [4, 0, 2, 0], leadingComments: ' The id.\n' },
        { path: [4, 0, 2, 5], leadingComments: ' Kept optional.\n' },
        { path: [4, 1], leadingComments: ' The response.\n' },
        { path: [4, 2], leadingComments: ' Internal request.\n' },
        { path: [5, 0, 2, 2], leadingComments: ' Secret value.\n' },
        { path: [6, 0, 2, 1], leadingComments: ' Preview method.\n' },
        { path: [6, 0, 2, 2], leadingComments: ' Internal method.\n' },
        { path: [6, 2], leadingComments: ' Dead service.\n' },
        { path: [3, 0], leadingComments: ' dep import.\n' }
      ]
    }
  }
  const depFile = {
    name: 'dep.proto',
    package: 'estest.dep',
    dependency: [],
    messageType: [
      {
        name: 'Shared',
        field: [str('v', 1)],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      },
      {
        name: 'Orphan',
        field: [str('w', 1)],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      }
    ],
    enumType: [],
    service: [],
    extension: []
  }
  const visFile = {
    name: 'google/api/visibility.proto',
    package: 'google.api',
    dependency: [],
    messageType: [
      {
        name: 'VisibilityRule',
        field: [str('selector', 1), str('restriction', 2)],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      }
    ],
    enumType: [],
    service: [],
    extension: []
  }
  return {
    fileToGenerate: ['svc.proto', 'dep.proto'],
    protoFile: [svcFile, depFile, visFile],
    sourceFileDescriptors: [structuredClone(svcFile), structuredClone(depFile)]
  }
}

const names = (arr) => (arr ?? []).map((x) => x.name)

test('PUBLIC surface: methods, services, fields, values, reachability', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PUBLIC)
  const [svc, dep] = req.protoFile

  // services: only PublicApi survives, with only the public method
  assert.deepEqual(names(svc.service), ['PublicApi'])
  assert.deepEqual(names(svc.service[0].method), ['Get'])

  // messages: internal-method closure (AdminReq/Detail) and Unused pruned
  assert.deepEqual(names(svc.messageType), ['Req', 'Resp'])
  assert.deepEqual(names(svc.messageType[0].nestedType), [
    'SharedEntry',
    'Used'
  ])

  // fields: internal ones removed
  assert.deepEqual(names(svc.messageType[0].field), [
    'id',
    'a',
    'kept',
    'shared',
    'kind',
    'used'
  ])

  // oneofs: 'sel' kept (one member dropped), '_gone' pruned, '_kept' reindexed
  assert.deepEqual(names(svc.messageType[0].oneofDecl), ['sel', '_kept'])
  const fieldByName = Object.fromEntries(
    svc.messageType[0].field.map((f) => [f.name, f])
  )
  assert.equal(fieldByName.a.oneofIndex, 0)
  assert.equal(fieldByName.kept.oneofIndex, 1)
  assert.equal(fieldByName.id.oneofIndex, undefined)

  // enums: internal value dropped, unreferenced enum pruned
  assert.deepEqual(names(svc.enumType), ['Kind'])
  assert.deepEqual(names(svc.enumType[0].value), ['KIND_UNSPECIFIED', 'KIND_A'])

  // extensions never survive
  assert.equal(svc.extension.length, 0)

  // dep.proto (also generated): only the referenced message survives
  assert.deepEqual(names(dep.messageType), ['Shared'])

  // dependency list recomputed: visibility.proto (options-only) dropped
  assert.deepEqual(svc.dependency, ['dep.proto'])

  // sourceFileDescriptors got the same rewrite
  assert.deepEqual(names(req.sourceFileDescriptors[0].service), ['PublicApi'])
  assert.deepEqual(names(req.sourceFileDescriptors[0].messageType), [
    'Req',
    'Resp'
  ])
})

test('PREVIEW surface keeps preview methods', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PREVIEW)
  assert.deepEqual(names(req.protoFile[0].service[0].method), ['Get', 'Peek'])
})

test('custom option bytes: only google.api.http survives, on methods only', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PUBLIC)
  const svc = req.protoFile[0]
  const get = svc.service[0].method[0]
  assert.deepEqual(
    get.options.$unknown.map((f) => f.no),
    [HTTP_EXT]
  )
  assert.equal(svc.options.$unknown, undefined)
  assert.equal(svc.options.javaPackage, 'com.estest') // known options stay
  for (const f of svc.messageType[0].field)
    assert.equal(f.options?.$unknown, undefined)
  for (const v of svc.enumType[0].value)
    assert.equal(v.options?.$unknown, undefined)
})

test('source info: dropped elements lose locations, survivors are remapped and scrubbed', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PUBLIC)
  const locs = req.protoFile[0].sourceCodeInfo.location
  const byPath = new Map(locs.map((l) => [l.path.join('.'), l]))

  // Req comment survives with the internal span scrubbed
  assert.equal(byPath.get('4.0').leadingComments, ' Request. Keep this.\n')
  // field id keeps its path; field 'kept' shifted 5 -> 2
  assert.ok(byPath.has('4.0.2.0'))
  assert.equal(byPath.get('4.0.2.2').leadingComments, ' Kept optional.\n')
  // AdminReq (4,2), internal/preview methods, DeadApi and the internal enum
  // value lost their locations
  assert.equal(byPath.has('4.2'), false)
  assert.equal(byPath.has('6.0.2.1'), false)
  assert.equal(byPath.has('6.0.2.2'), false)
  assert.equal(byPath.has('6.2'), false)
  assert.equal(byPath.has('5.0.2.2'), false)
  // Resp shifted from 4,1 — stays 4,1 (Req kept before it)
  assert.equal(byPath.get('4.1').leadingComments, ' The response.\n')
  // dependency comment survives (dep.proto kept at index 0)
  assert.ok(byPath.has('3.0'))
})

test('PREVIEW surface: preview annotations dropped from option bytes too', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PREVIEW)
  const peek = req.protoFile[0].service[0].method[1]
  assert.equal(peek.options?.$unknown, undefined)
})

test('contradiction: internal type reachable from public surface throws', () => {
  const req = makeReq()
  req.protoFile[0].messageType[1].options = { $unknown: [vis('INTERNAL')] } // Resp
  assert.throws(
    () => applyVisibilitySurface(req, PUBLIC),
    /'estest\.Resp' is restricted below/
  )
})

test('enum left empty by value filtering throws', () => {
  const req = makeReq()
  for (const v of req.protoFile[0].enumType[0].value) {
    v.options = { $unknown: [vis('INTERNAL')] }
  }
  assert.throws(
    () => applyVisibilitySurface(req, PUBLIC),
    /every value of enum 'estest\.Kind'/
  )
})

test('unknown restriction label throws', () => {
  const req = makeReq()
  req.protoFile[0].service[0].method[0].options.$unknown.push(vis('PUBILC'))
  assert.throws(
    () => applyVisibilitySurface(req, PUBLIC),
    /unknown google.api visibility restriction 'PUBILC'/
  )
})

test('multi-label restriction takes its highest label', () => {
  const req = makeReq()
  req.protoFile[0].service[0].method[2].options.$unknown = [
    vis('INTERNAL,PUBLIC')
  ]
  applyVisibilitySurface(req, PUBLIC)
  assert.deepEqual(names(req.protoFile[0].service[0].method), ['Get', 'Admin'])
})

test('import public is rejected', () => {
  const req = makeReq()
  req.protoFile[0].publicDependency = [0]
  assert.throws(
    () => applyVisibilitySurface(req, PUBLIC),
    /does not support 'import public'/
  )
})

test('files not in file_to_generate are pruned in-memory to the same closure', () => {
  const req = makeReq()
  req.fileToGenerate = ['svc.proto']
  applyVisibilitySurface(req, PUBLIC)
  // dep.proto is not emitted, but its in-memory copy is pruned all the same so
  // the registry protoc-gen-es builds from the request stays consistent
  assert.deepEqual(names(req.protoFile[1].messageType), ['Shared'])
  // and svc.proto still keeps its dependency on it
  assert.deepEqual(req.protoFile[0].dependency, ['dep.proto'])
})

test('bystander dep referencing a pruned generated type is pruned, not dangling', () => {
  // Production shape: gen web imports non-gen project imports gen common; the
  // public surface reaches none of project's types, but project's fields
  // reference common types that pruning removes. The bystander must be pruned
  // too, or protoc-gen-es's registry build dies on the dangling reference.
  const req = makeReq()
  req.protoFile[0].dependency.push('bystander.proto')
  req.protoFile.push({
    name: 'bystander.proto',
    package: 'estest.by',
    dependency: ['svc.proto'],
    messageType: [
      {
        name: 'Watcher',
        // references a type that the PUBLIC surface prunes from svc.proto
        field: [msgField('admin', 1, '.estest.AdminReq')],
        nestedType: [],
        enumType: [],
        oneofDecl: [],
        extension: []
      }
    ],
    enumType: [],
    service: [
      // a bystander's service is never emitted: dropped outright, so its
      // method types do not anchor anything into the closure
      {
        name: 'WatcherApi',
        method: [method('Watch', '.estest.by.Watcher', '.estest.by.Watcher')]
      }
    ],
    extension: []
  })
  applyVisibilitySurface(req, PUBLIC)
  const by = req.protoFile.at(-1)
  assert.deepEqual(names(by.messageType), [])
  assert.deepEqual(names(by.service), [])
  // the generated file no longer depends on the emptied bystander
  assert.deepEqual(req.protoFile[0].dependency, ['dep.proto'])
})

test('a service whose every method is internal disappears, name and all', () => {
  const req = makeReq()
  applyVisibilitySurface(req, PUBLIC)
  const out = JSON.stringify(req.protoFile[0])
  assert.equal(out.includes('DeadApi'), false)
  assert.equal(out.includes('HiddenApi'), false)
  assert.equal(out.includes('AdminReq'), false)
  assert.equal(out.includes('my_ext'), false)
})

test('restrictionOf reads the last restriction of merged option bytes', () => {
  const options = { $unknown: [vis('INTERNAL'), vis('PREVIEW')] }
  assert.equal(restrictionOf(options), 'PREVIEW')
  assert.equal(restrictionOf({}), undefined)
})

test('scrubComment strips spans and unterminated markers', () => {
  assert.equal(scrubComment('Keep. (-- drop --) More.'), 'Keep. More.')
  assert.equal(
    scrubComment('Keep.\n(-- drop\nmultiline --)\nMore.'),
    'Keep.\n\nMore.'
  )
  assert.equal(scrubComment('Keep. (-- never closed'), 'Keep.')
  assert.equal(scrubComment('(-- all internal --)'), '')
})

test('type reference outside the request throws', () => {
  const req = makeReq()
  req.protoFile[0].service[0].method[0].inputType = '.estest.Missing'
  assert.throws(
    () => applyVisibilitySurface(req, PUBLIC),
    /'estest\.Missing' .* not found/
  )
})
