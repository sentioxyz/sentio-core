import type { Story } from '@ladle/react'
import MonacoEditor from '@monaco-editor/react'
import { sentioTheme } from './SentioTheme'
import { SourceTree, TreeNode } from './SourceTree'
import { useState } from 'react'
import '../styles.css'

const sourceCode = `/**
 * Submitted for verification at Etherscan.io on 20XX-XX-XX
 */

pragma solidity ^0.4.24;

// File: zos-lib/contracts/upgradeability/Proxy.sol

/**
 * @title Proxy
 * @dev Implements delegation of calls to other contracts, with proper
 * forwarding of return values and bubbling of failures.
 * It defines a fallback function that delegates all calls to the address
 * returned by the abstract _implementation() internal function.
 */
contract Proxy {
  /**
   * @dev Fallback function.
   * Implemented entirely in \`_fallback\`.
   */
  function () payable external {
    _fallback();
  }

  /**
   * @return The Address of the implementation.
   */
  function _implementation() internal view returns (address);

  /**
   * @dev Delegates execution to an implementation contract.
   * This is a low level function that doesn't return to its internal call site.
   * It will return to the external caller whatever the implementation returns.
   * @param implementation Address to delegate.
   */
  function _delegate(address implementation) internal {
    assembly {
      // Copy msg.data. We take full control of memory in this inline assembly
      // block because it will not return to Solidity code. We overwrite the
      // Solidity scratch pad at memory position 0.
      calldatacopy(0, 0, calldatasize)

      // Call the implementation.
      // out and outsize are 0 because we don't know the size yet.
      let result := delegatecall(gas, implementation, 0, calldatasize, 0, 0)

      // Copy the returned data.
      returndatacopy(0, 0, returndatasize)

      switch result
      // delegatecall returns 0 on error.
      case 0 { revert(0, returndatasize) }
      default { return(0, returndatasize) }
    }
  }

  /**
   * @dev Function that is run as the first thing in the fallback function.
   * Can be redefined in derived contracts to add functionality.
   * Redefinitions must call super._willFallback().
   */
  function _willFallback() internal {
  }

  /**
   * @dev fallback implementation.
   * Extracted to enable manual triggering.
   */
  function _fallback() internal {
    _willFallback();
    _delegate(_implementation());
  }
}`

export const EditorTheme: Story = () => (
  <div style={{ height: '80vh' }} className="rounded-md border p-px">
    <MonacoEditor
      language="sol"
      theme="sentio"
      value={sourceCode}
      onMount={(editor, monaco) => {
        monaco.editor.defineTheme('sentio', sentioTheme)
      }}
    />
  </div>
)

const mockTreeData: TreeNode = {
  text: 'contracts',
  path: 'contracts',
  children: [
    {
      text: 'upgradeability',
      path: 'contracts/upgradeability',
      children: [
        {
          text: 'Proxy.sol',
          path: 'contracts/upgradeability/Proxy.sol',
          children: []
        },
        {
          text: 'UpgradeableProxy.sol',
          path: 'contracts/upgradeability/UpgradeableProxy.sol',
          children: []
        }
      ]
    },
    {
      text: 'token',
      path: 'contracts/token',
      children: [
        {
          text: 'ERC20.sol',
          path: 'contracts/token/ERC20.sol',
          children: []
        }
      ]
    }
  ]
}

export const SourceTreeExample: Story = () => {
  const [selectedPath, setSelectedPath] = useState(
    'contracts/upgradeability/Proxy.sol'
  )

  return (
    <div style={{ padding: '20px', maxWidth: '400px' }}>
      <h3>Source Tree</h3>
      <SourceTree
        node={mockTreeData}
        selectedPath={selectedPath}
        onSelect={setSelectedPath}
      />
      <p style={{ marginTop: '20px' }}>Selected: {selectedPath}</p>
    </div>
  )
}
