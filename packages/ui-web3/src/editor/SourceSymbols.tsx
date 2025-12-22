import { FlatTree, DataNode, classNames } from '@sentio/ui-core'
import { OutlineTree, SymbolInformationKind as SIK } from '@sentio/scip'
import { memo, useMemo, useState } from 'react'
import { DocumentTextIcon } from '@heroicons/react/20/solid'
import { DebounceInput } from 'react-debounce-input'
import { useResizeDetector } from 'react-resize-detector'

export const SymbolIcons = {
  [SIK.Array]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Array"
    >
      A
    </span>
  ),
  [SIK.Assertion]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Assertion"
    >
      A
    </span>
  ),
  [SIK.AssociatedType]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="AssociatedType"
    >
      A
    </span>
  ),
  [SIK.Attribute]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Attribute"
    >
      A
    </span>
  ),
  [SIK.Axiom]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Axiom"
    >
      A
    </span>
  ),
  [SIK.Boolean]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Boolean"
    >
      B
    </span>
  ),
  [SIK.Class]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-orange-600 text-center text-white"
      title="Class"
    >
      C
    </span>
  ),
  [SIK.Constant]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Constant"
    >
      C
    </span>
  ),
  [SIK.Constructor]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Constructor"
    >
      C
    </span>
  ),
  [SIK.DataFamily]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="DataFamily"
    >
      D
    </span>
  ),
  [SIK.Enum]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Enum"
    >
      E
    </span>
  ),
  [SIK.EnumMember]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="EnumMember"
    >
      E
    </span>
  ),
  [SIK.Event]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Event"
    >
      E
    </span>
  ),
  [SIK.Fact]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Fact"
    >
      F
    </span>
  ),
  [SIK.Field]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Field"
    >
      F
    </span>
  ),
  [SIK.File]: <DocumentTextIcon className="inline-block h-4 w-4" />,
  [SIK.Function]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Function"
    >
      F
    </span>
  ),
  [SIK.Getter]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Getter"
    >
      G
    </span>
  ),
  [SIK.Grammar]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Grammar"
    >
      G
    </span>
  ),
  [SIK.Instance]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Instance"
    >
      I
    </span>
  ),
  [SIK.Interface]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Interface"
    >
      I
    </span>
  ),
  [SIK.Key]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Key"
    >
      K
    </span>
  ),
  [SIK.Lang]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Lang"
    >
      L
    </span>
  ),
  [SIK.Lemma]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Lemma"
    >
      L
    </span>
  ),
  [SIK.Macro]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Macro"
    >
      M
    </span>
  ),
  [SIK.Method]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Method"
    >
      M
    </span>
  ),
  [SIK.MethodReceiver]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="MethodReceiver"
    >
      M
    </span>
  ),
  [SIK.Message]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Message"
    >
      M
    </span>
  ),
  [SIK.Module]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Module"
    >
      M
    </span>
  ),
  [SIK.Namespace]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Namespace"
    >
      N
    </span>
  ),
  [SIK.Null]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Null"
    >
      N
    </span>
  ),
  [SIK.Object]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Object"
    >
      O
    </span>
  ),
  [SIK.Operator]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Operator"
    >
      O
    </span>
  ),
  [SIK.Package]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Package"
    >
      P
    </span>
  ),
  [SIK.PackageObject]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="PackageObject"
    >
      P
    </span>
  ),
  [SIK.Parameter]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Parameter"
    >
      P
    </span>
  ),
  [SIK.ParameterLabel]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="ParameterLabel"
    >
      P
    </span>
  ),
  [SIK.Pattern]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Pattern"
    >
      P
    </span>
  ),
  [SIK.Predicate]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Predicate"
    >
      P
    </span>
  ),
  [SIK.Property]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Property"
    >
      P
    </span>
  ),
  [SIK.Protocol]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Protocol"
    >
      P
    </span>
  ),
  [SIK.Quasiquoter]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Quasiquoter"
    >
      Q
    </span>
  ),
  [SIK.SelfParameter]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="SelfParameter"
    >
      S
    </span>
  ),
  [SIK.Setter]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Setter"
    >
      S
    </span>
  ),
  [SIK.Signature]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Signature"
    >
      S
    </span>
  ),
  [SIK.Subscript]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Subscript"
    >
      S
    </span>
  ),
  [SIK.String]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="String"
    >
      S
    </span>
  ),
  [SIK.Struct]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Struct"
    >
      S
    </span>
  ),
  [SIK.Tactic]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Tactic"
    >
      T
    </span>
  ),
  [SIK.Theorem]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Theorem"
    >
      T
    </span>
  ),
  [SIK.ThisParameter]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="ThisParameter"
    >
      T
    </span>
  ),
  [SIK.Trait]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Trait"
    >
      T
    </span>
  ),
  [SIK.Type]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Type"
    >
      T
    </span>
  ),
  [SIK.TypeAlias]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="TypeAlias"
    >
      T
    </span>
  ),
  [SIK.TypeClass]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="TypeClass"
    >
      T
    </span>
  ),
  [SIK.TypeFamily]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="TypeFamily"
    >
      T
    </span>
  ),
  [SIK.TypeParameter]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="TypeParameter"
    >
      T
    </span>
  ),
  [SIK.Union]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Union"
    >
      U
    </span>
  ),
  [SIK.Value]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Value"
    >
      V
    </span>
  ),
  [SIK.Variable]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Variable"
    >
      V
    </span>
  ),
  [SIK.Contract]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-purple-600 text-center text-white"
      title="Contract"
    >
      C
    </span>
  ),
  [SIK.Library]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-orange-600 text-center text-white"
      title="Library"
    >
      L
    </span>
  ),
  [SIK.Modifier]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-orange-600 text-center text-white"
      title="Modifier"
    >
      M
    </span>
  ),
  [SIK.Error]: (
    <span
      className="bg-magenta-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Error"
    >
      E
    </span>
  ),
  [SIK.UnspecifiedKind]: (
    <span
      className="inline-block h-4 w-4 rounded-full bg-orange-600 text-center text-white"
      title="UnspecifiedKind"
    >
      U
    </span>
  ),
  [SIK.Number]: (
    <span
      className="bg-daybreak-blue-600 inline-block h-4 w-4 rounded-full text-center text-white"
      title="Number"
    >
      N
    </span>
  )
}

interface Props {
  data: OutlineTree[]
  onClick: (range: monaco.IRange) => void
}

const SourceSymbols = ({ data, onClick }: Props) => {
  const [searchKey, setSearchKey] = useState('')
  const treeNode = useMemo(() => {
    const hasKeyFilter = searchKey !== ''
    const lowerSearchKey = searchKey.toLowerCase()
    const walkTree = (item: OutlineTree, depth = 0): DataNode | undefined => {
      const {
        symbol: { kind, symbol, displayName, signatureDocumentation },
        children
      } = item
      const node = {
        key: symbol,
        title: (
          <div className="group flex w-full cursor-pointer items-center gap-2">
            <span className="flex-0">{kind ? SymbolIcons[kind] : null}</span>
            <span
              className={classNames(
                'flex-0 underline-offset-2 group-hover:underline',
                displayName ? '' : 'text-gray-400'
              )}
            >
              {displayName || 'anonymous'}
            </span>
            <span className="text-gray-400">
              {signatureDocumentation?.text}
            </span>
          </div>
        ),
        depth: depth,
        raw: item
      } as DataNode
      if (children?.length > 0) {
        const filteredChildren = children
          .map((child) => walkTree(child, depth + 1))
          .filter((item) => item !== undefined)
        if (filteredChildren.length > 0) {
          node.children = filteredChildren as DataNode[]
        }
      }

      if (!hasKeyFilter) {
        return node
      }
      if (
        displayName?.toLowerCase().includes(lowerSearchKey) ||
        node.children
      ) {
        return node
      }
    }
    const rootNode = data
      .map((item) => walkTree(item))
      .filter((item) => item !== undefined) as DataNode[]
    return rootNode
  }, [data, searchKey])

  const symbolCount = useMemo(() => {
    let count = 0
    function walk(item?: DataNode) {
      if (!item) {
        return
      }
      if (item.children) {
        item.children.forEach(walk)
      } else {
        count++
      }
    }
    treeNode.forEach(walk)
    return count
  }, [treeNode])
  const { width, ref } = useResizeDetector({ handleHeight: false })

  return (
    <div ref={ref}>
      <div className="w-fit min-w-full">
        <div
          className="dark:bg-sentio-gray-100 sticky left-0 top-0 z-[1] space-y-2 bg-white py-2"
          style={{ width }}
        >
          <div className="relative px-2">
            <DebounceInput
              className="text-icontent focus-within:border-primary-500 w-full rounded-md  px-[10px] py-1.5"
              placeholder="Search symbols..."
              value={searchKey}
              onChange={(evt) => {
                const value = evt.target.value
                setSearchKey(value)
              }}
              onBlur={(evt) => {
                const value = evt.target.value
                setSearchKey(value)
              }}
              debounceTimeout={300}
              type="search"
            />
          </div>
          {searchKey ? (
            <div className="text-icontent text-gray">
              <span className="mr-2">{symbolCount} symbols matching</span>
              <span className="font-medium">{searchKey}</span>
            </div>
          ) : null}
        </div>
        <FlatTree
          defaultExpandAll
          data={treeNode}
          onClick={(item) => {
            const { range } = item.raw as OutlineTree
            onClick({
              startLineNumber: range!.start.line + 1,
              startColumn: range!.start.character + 1,
              endLineNumber: range!.end.line + 1,
              endColumn: range!.end.character + 1
            })
          }}
        />
        {searchKey ? null : (
          <div className="text-icontent text-gray px-2 py-1">
            {symbolCount} symbols total
          </div>
        )}
      </div>
    </div>
  )
}

export default memo(SourceSymbols)
