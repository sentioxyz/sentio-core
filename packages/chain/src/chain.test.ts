import { ChainId, EthChainId } from './chain-id'
import { ChainInfo, EthChainInfo } from './chain-info'
import { getChainName } from './chain-name'
import assert from 'assert'
import { describe, it, before, afterEach } from 'node:test'
import * as console from 'node:console'

describe('Chain Test', () => {
  it('chain name', () => {
    assert.equal(getChainName(ChainId.POLYGON_ZKEVM), 'Polygon zkEVM')
    assert.equal(getChainName(ChainId.ZKSYNC_ERA), 'zkSync Era')
    assert.equal(getChainName('592'), 'Astar')
  })

  it('chain id and map key matches', () => {
    assert.equal(
      Object.entries(ChainInfo).length,
      Object.entries(ChainId).length
    )
    for (const [key, info] of Object.entries(ChainInfo)) {
      assert.equal(key, info.chainId)
    }
  })

  it('eth chain name and chain id matches', () => {
    const idKeys = Object.values(EthChainId).sort()
    // const nameKeys = Object.keys(EthChainName).sort()
    const detailKeys = Object.values(EthChainInfo)
      .map((d) => d.chainId)
      .sort()

    // Make key and chain id matches
    for (const [key, value] of Object.entries(EthChainInfo)) {
      assert.equal(key, value.chainId)
    }

    assert.deepEqual(idKeys, detailKeys)
  })

  it('chain info map and chain id map list should have same keys ', () => {
    const keys1 = Object.values(ChainInfo)
      .map((info) => info.chainId)
      .sort()
    const keys2 = Object.values(ChainId).sort()

    let missing = keys1.filter((item) => keys2.indexOf(item) < 0)
    if (missing) {
      console.log('missing', missing)
    }
    missing = keys2.filter((item) => keys1.indexOf(item) < 0)
    if (missing) {
      console.log('missing', missing)
    }
    assert.deepEqual(keys1, keys2)
  })
})
