import '../styles.css'
import { BalanceChanges } from './BalanceChanges'
import {
  CallTracesContext,
  ChainIdContext,
  PriceFetcherContext
} from './transaction-context'
import { SvgFolderContext } from '../utils/extension-context'

const callTraceData = {
  calls: [
    {
      calls: [
        {
          calls: [],
          contractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          depth: 2,
          dynamicLogs: [],
          endIndex: 2103,
          from: '0x000000000022d473030f116ddee9f6b43ac78ba3',
          fromContractName: '',
          functionName: 'transferFrom',
          gas: '0x2645e',
          gasUsed: '0x75c9',
          inputs: [
            {
              name: 'from',
              type: 'address',
              value: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d'
            },
            {
              name: 'to',
              type: 'address',
              value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
            },
            {
              name: 'value',
              type: 'uint256',
              value: '240000000000000000000000000000'
            }
          ],
          location: {
            instructionIndex: 1210
          },
          logs: [
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              data: '0x0000000000000000000000000000000000000003077b58d5d378391980000000',
              endIndex: 1814,
              events: [
                {
                  name: 'from',
                  type: 'address',
                  value: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d'
                },
                {
                  name: 'to',
                  type: 'address',
                  value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
                },
                {
                  name: 'value',
                  type: 'uint256',
                  value: '240000000000000000000000000000'
                }
              ],
              location: {
                instructionIndex: 1813
              },
              name: 'Transfer',
              startIndex: 1813,
              topics: [
                '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
                '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21'
              ]
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              data: '0xffffffffffffffffffffffffffffffffffffffc1418a7215e8aadd090ad4d567',
              endIndex: 2030,
              events: [
                {
                  name: 'owner',
                  type: 'address',
                  value: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d'
                },
                {
                  name: 'spender',
                  type: 'address',
                  value: '0x000000000022d473030f116ddee9f6b43ac78ba3'
                },
                {
                  name: 'value',
                  type: 'uint256',
                  value:
                    '115792089237316195423570985008687907853269984660669473697214351116094282192231'
                }
              ],
              location: {
                instructionIndex: 2029
              },
              name: 'Approval',
              startIndex: 2029,
              topics: [
                '0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925',
                '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                '0x000000000000000000000000000000000022d473030f116ddee9f6b43ac78ba3'
              ]
            }
          ],
          rawInput:
            '0x23b872dd0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df210000000000000000000000000000000000000003077b58d5d378391980000000',
          rawOutput:
            '0x0000000000000000000000000000000000000000000000000000000000000001',
          returnValue: [
            {
              name: '',
              type: 'bool',
              value: true
            }
          ],
          startIndex: 1210,
          storages: [
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1518,
              mappingKeys: [
                {
                  baseSlot:
                    '0x000000000000000000000000000000000000000000000000000000000000000a',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0xa876ba55103ba08fb7d2caad5d52d7f16d8347448233e3f91c99ae6ee9456bd3',
                  offset: 0,
                  slot: '0xa876ba55103ba08fb7d2caad5d52d7f16d8347448233e3f91c99ae6ee9456bd3'
                }
              ],
              slot: '0xa876ba55103ba08fb7d2caad5d52d7f16d8347448233e3f91c99ae6ee9456bd3',
              startIndex: 1517,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000000000000000000000'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1552,
              mappingKeys: [
                {
                  baseSlot:
                    '0x000000000000000000000000000000000000000000000000000000000000000a',
                  key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                  keySlot:
                    '0x6f9fc27356768393ec4aa5752cfe4027735a0dd8043291d2ac8129c887d1b6ef',
                  offset: 0,
                  slot: '0x6f9fc27356768393ec4aa5752cfe4027735a0dd8043291d2ac8129c887d1b6ef'
                }
              ],
              slot: '0x6f9fc27356768393ec4aa5752cfe4027735a0dd8043291d2ac8129c887d1b6ef',
              startIndex: 1551,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000000000000000000000'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1571,
              mappingKeys: [],
              slot: '0x0000000000000000000000000000000000000000000000000000000000000009',
              startIndex: 1570,
              type: 'SLOAD',
              value:
                '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1588,
              mappingKeys: [],
              slot: '0x0000000000000000000000000000000000000000000000000000000000000006',
              startIndex: 1587,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000000000000000000000'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1630,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000001',
                  key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                  keySlot:
                    '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
                  offset: 0,
                  slot: '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5'
                }
              ],
              slot: '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
              startIndex: 1629,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000003077b58d5d378391980000000'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1663,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000001',
                  key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                  keySlot:
                    '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
                  offset: 0,
                  slot: '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5'
                }
              ],
              slot: '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
              startIndex: 1662,
              type: 'SSTORE',
              value:
                '0x0000000000000000000000000000000000000000000000000000000000000000'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1687,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000001',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
                  offset: 0,
                  slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995'
                }
              ],
              slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
              startIndex: 1686,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000217ebe63a2743286801cae23364'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1750,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000001',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
                  offset: 0,
                  slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995'
                }
              ],
              slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
              startIndex: 1749,
              type: 'SSTORE',
              value:
                '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae23364'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1879,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000002',
                  key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                  keySlot:
                    '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8',
                  offset: 0,
                  slot: '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8'
                },
                {
                  baseSlot:
                    '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8',
                  key: '0x000000000000000000000000000000000022d473030f116ddee9f6b43ac78ba3',
                  keySlot:
                    '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3',
                  offset: 0,
                  slot: '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3'
                }
              ],
              slot: '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3',
              startIndex: 1878,
              type: 'SLOAD',
              value:
                '0xffffffffffffffffffffffffffffffffffffffc44905caebbc2316228ad4d567'
            },
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 1966,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000002',
                  key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
                  keySlot:
                    '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8',
                  offset: 0,
                  slot: '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8'
                },
                {
                  baseSlot:
                    '0x9aa9572d9a41b3fa32249362b6def85ee5b0fa5ff2b93df7474888efbf403ec8',
                  key: '0x000000000000000000000000000000000022d473030f116ddee9f6b43ac78ba3',
                  keySlot:
                    '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3',
                  offset: 0,
                  slot: '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3'
                }
              ],
              slot: '0xe765ddc9437e2224d5f3ca6b874152103d6f9131bb86190bf015ab474ba55ed3',
              startIndex: 1965,
              type: 'SSTORE',
              value:
                '0xffffffffffffffffffffffffffffffffffffffc1418a7215e8aadd090ad4d567'
            }
          ],
          to: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          toContractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          type: 'CALL',
          value: '0x0'
        }
      ],
      contractName: '0x000000000022d473030f116ddee9f6b43ac78ba3',
      depth: 1,
      dynamicLogs: [],
      endIndex: 2124,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'transferFrom',
      gas: '0x2841a',
      gasUsed: '0x8c14',
      inputs: [
        {
          name: 'from',
          type: 'address',
          value: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d'
        },
        {
          name: 'to',
          type: 'address',
          value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
        },
        {
          name: 'amount',
          type: 'uint160',
          value: '240000000000000000000000000000'
        },
        {
          name: 'token',
          type: 'address',
          value: '0x082646b22a3960da69ef7a778c16dd6fb85dd999'
        }
      ],
      location: {
        instructionIndex: 983
      },
      logs: [],
      rawInput:
        '0x36c785160000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df210000000000000000000000000000000000000003077b58d5d378391980000000000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999',
      rawOutput: '0x',
      startIndex: 983,
      storages: [
        {
          address: '0x000000000022d473030f116ddee9f6b43ac78ba3',
          endIndex: 1150,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000001',
              key: '0x0000000000000000000000005e8bb488e85ea732e17150862b1acfc213a7c13d',
              keySlot:
                '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
              offset: 0,
              slot: '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5'
            },
            {
              baseSlot:
                '0x0e0734b0928549d1def508ef54161cb759e48a8efc012b75c23359d4353653f5',
              key: '0x000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999',
              keySlot:
                '0x35e9493ce45bc7b062a3736d8594675b670bd1788353ed7b91c31a2453cd3570',
              offset: 0,
              slot: '0x35e9493ce45bc7b062a3736d8594675b670bd1788353ed7b91c31a2453cd3570'
            },
            {
              baseSlot:
                '0x35e9493ce45bc7b062a3736d8594675b670bd1788353ed7b91c31a2453cd3570',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x8b417a1d37a8ac5c40902161a052e9cd68e9734136b8449dd931112c2e7f222c',
              offset: 0,
              slot: '0x8b417a1d37a8ac5c40902161a052e9cd68e9734136b8449dd931112c2e7f222c'
            }
          ],
          slot: '0x8b417a1d37a8ac5c40902161a052e9cd68e9734136b8449dd931112c2e7f222c',
          startIndex: 1149,
          type: 'SLOAD',
          value:
            '0x00000000000200006490727cffffffffffffffffffffffffffffffffffffffff'
        }
      ],
      to: '0x000000000022d473030f116ddee9f6b43ac78ba3',
      toContractName: '0x000000000022d473030f116ddee9f6b43ac78ba3',
      type: 'CALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      depth: 1,
      dynamicLogs: [],
      endIndex: 2350,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'balanceOf',
      gas: '0x1eea6',
      gasUsed: '0x9e6',
      inputs: [
        {
          name: 'account',
          type: 'address',
          value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
        }
      ],
      location: {
        instructionIndex: 2240
      },
      logs: [],
      rawInput:
        '0x70a08231000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      rawOutput:
        '0x0000000000000000000000000000000000000000000000000000000000000000',
      returnValue: [
        {
          name: '',
          type: 'uint256',
          value: '0'
        }
      ],
      startIndex: 2240,
      storages: [
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 2329,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 2328,
          type: 'SLOAD',
          value:
            '0x0000000000000000000000000000000000000000000000000000000000000000'
        }
      ],
      to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      type: 'STATICCALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      depth: 1,
      dynamicLogs: [],
      endIndex: 2748,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'getReserves',
      gas: '0x1d725',
      gasUsed: '0x9c8',
      inputs: [],
      location: {
        instructionIndex: 2636
      },
      logs: [],
      rawInput: '0x0902f1ac',
      rawOutput:
        '0x0000000000000000000000000000000000000217ebe63a2743286801cae233640000000000000000000000000000000000000000000000023723fadaf096622600000000000000000000000000000000000000000000000000000000647df6e3',
      returnValue: [
        {
          name: '_reserve0',
          type: 'uint112',
          value: '42460074249999466325877131981668'
        },
        {
          name: '_reserve1',
          type: 'uint112',
          value: '40866783261936214566'
        },
        {
          name: '_blockTimestampLast',
          type: 'uint32',
          value: '1685976803'
        }
      ],
      startIndex: 2636,
      storages: [
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 2695,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000008',
          startIndex: 2694,
          type: 'SLOAD',
          value:
            '0x647df6e30000000000023723fadaf09662260217ebe63a2743286801cae23364'
        }
      ],
      to: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      toContractName: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      type: 'STATICCALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
      depth: 1,
      dynamicLogs: [],
      endIndex: 3168,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'balanceOf',
      gas: '0x1ca89',
      gasUsed: '0x376',
      inputs: [
        {
          name: 'account',
          type: 'address',
          value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
        }
      ],
      location: {
        instructionIndex: 2947
      },
      logs: [],
      rawInput:
        '0x70a08231000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
      rawOutput:
        '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae23364',
      returnValue: [
        {
          name: '',
          type: 'uint256',
          value: '42700074249999466325877131981668'
        }
      ],
      startIndex: 2947,
      storages: [
        {
          address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          endIndex: 3106,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000001',
              key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
              keySlot:
                '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
              offset: 0,
              slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995'
            }
          ],
          slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
          startIndex: 3105,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae23364'
        }
      ],
      to: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
      toContractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
      type: 'STATICCALL',
      value: '0x0'
    },
    {
      calls: [
        {
          calls: [],
          contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          depth: 2,
          dynamicLogs: [],
          endIndex: 4217,
          from: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          fromContractName: '',
          functionName: 'transfer',
          gas: '0x190c7',
          gasUsed: '0x6d3a',
          inputs: [
            {
              name: 'to',
              type: 'address',
              value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
            },
            {
              name: 'value',
              type: 'uint256',
              value: '229010580386381830'
            }
          ],
          location: {
            instructionIndex: 3944
          },
          logs: [
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              data: '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006',
              endIndex: 4177,
              events: [
                {
                  name: 'from',
                  type: 'address',
                  value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
                },
                {
                  name: 'to',
                  type: 'address',
                  value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
                },
                {
                  name: 'value',
                  type: 'uint256',
                  value: '229010580386381830'
                }
              ],
              location: {
                instructionIndex: 4176
              },
              name: 'Transfer',
              startIndex: 4176,
              topics: [
                '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef',
                '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
              ]
            }
          ],
          rawInput:
            '0xa9059cbb000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b000000000000000000000000000000000000000000000000032d9be8d5bc8006',
          rawOutput:
            '0x0000000000000000000000000000000000000000000000000000000000000001',
          returnValue: [
            {
              name: '',
              type: 'bool',
              value: true
            }
          ],
          startIndex: 3944,
          storages: [
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4065,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
                  offset: 0,
                  slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a'
                }
              ],
              slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
              startIndex: 4064,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000023723fadaf0966226'
            },
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4112,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
                  offset: 0,
                  slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a'
                }
              ],
              slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
              startIndex: 4111,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000023723fadaf0966226'
            },
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4119,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
                  offset: 0,
                  slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a'
                }
              ],
              slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
              startIndex: 4118,
              type: 'SSTORE',
              value:
                '0x00000000000000000000000000000000000000000000000233f65ef21ad9e220'
            },
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4143,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
                  keySlot:
                    '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
                  offset: 0,
                  slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
                }
              ],
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              startIndex: 4142,
              type: 'SLOAD',
              value:
                '0x0000000000000000000000000000000000000000000000000000000000000000'
            },
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4150,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
                  keySlot:
                    '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
                  offset: 0,
                  slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
                }
              ],
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              startIndex: 4149,
              type: 'SSTORE',
              value:
                '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006'
            }
          ],
          to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          type: 'CALL',
          value: '0x0'
        },
        {
          calls: [],
          contractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          depth: 2,
          dynamicLogs: [],
          endIndex: 4568,
          from: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          fromContractName: '',
          functionName: 'balanceOf',
          gas: '0x122e8',
          gasUsed: '0x376',
          inputs: [
            {
              name: 'account',
              type: 'address',
              value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
            }
          ],
          location: {
            instructionIndex: 4347
          },
          logs: [],
          rawInput:
            '0x70a08231000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
          rawOutput:
            '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae23364',
          returnValue: [
            {
              name: '',
              type: 'uint256',
              value: '42700074249999466325877131981668'
            }
          ],
          startIndex: 4347,
          storages: [
            {
              address: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
              endIndex: 4506,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000001',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
                  offset: 0,
                  slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995'
                }
              ],
              slot: '0x10e08021664e63029570c8bae88bd9b0876177666c6cadb2f5215b4585c2a995',
              startIndex: 4505,
              type: 'SLOAD',
              value:
                '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae23364'
            }
          ],
          to: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          toContractName: '0x082646b22a3960da69ef7a778c16dd6fb85dd999',
          type: 'STATICCALL',
          value: '0x0'
        },
        {
          calls: [],
          contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          depth: 2,
          dynamicLogs: [],
          endIndex: 4749,
          from: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          fromContractName: '',
          functionName: 'balanceOf',
          gas: '0x11deb',
          gasUsed: '0x216',
          inputs: [
            {
              name: 'account',
              type: 'address',
              value: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21'
            }
          ],
          location: {
            instructionIndex: 4639
          },
          logs: [],
          rawInput:
            '0x70a08231000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
          rawOutput:
            '0x00000000000000000000000000000000000000000000000233f65ef21ad9e220',
          returnValue: [
            {
              name: '',
              type: 'uint256',
              value: '40637772681549832736'
            }
          ],
          startIndex: 4639,
          storages: [
            {
              address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
              endIndex: 4728,
              mappingKeys: [
                {
                  baseSlot:
                    '0x0000000000000000000000000000000000000000000000000000000000000003',
                  key: '0x000000000000000000000000f8956e715b9aa5897c6e81ce50b4c7256f43df21',
                  keySlot:
                    '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
                  offset: 0,
                  slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a'
                }
              ],
              slot: '0x07a0f8374686519308ecef8d6b5bac16a84e55cbe378b5450db034b0996ab58a',
              startIndex: 4727,
              type: 'SLOAD',
              value:
                '0x00000000000000000000000000000000000000000000000233f65ef21ad9e220'
            }
          ],
          to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          type: 'STATICCALL',
          value: '0x0'
        }
      ],
      contractName: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      depth: 1,
      dynamicLogs: [],
      endIndex: 5477,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'swap',
      gas: '0x1c1bd',
      gasUsed: '0xed21',
      inputs: [
        {
          name: 'amount0Out',
          type: 'uint256',
          value: '0'
        },
        {
          name: 'amount1Out',
          type: 'uint256',
          value: '229010580386381830'
        },
        {
          name: 'to',
          type: 'address',
          value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
        },
        {
          name: 'data',
          type: 'bytes',
          value: null
        }
      ],
      location: {
        instructionIndex: 3507
      },
      logs: [
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          data: '0x000000000000000000000000000000000000021af36192fd16a0a11b4ae2336400000000000000000000000000000000000000000000000233f65ef21ad9e220',
          endIndex: 5411,
          events: [
            {
              name: 'reserve0',
              type: 'uint112',
              value: '42700074249999466325877131981668'
            },
            {
              name: 'reserve1',
              type: 'uint112',
              value: '40637772681549832736'
            }
          ],
          location: {
            instructionIndex: 5410
          },
          name: 'Sync',
          startIndex: 5410,
          topics: [
            '0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1'
          ]
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          data: '0x0000000000000000000000000000000000000003077b58d5d37839198000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000032d9be8d5bc8006',
          endIndex: 5460,
          events: [
            {
              name: 'sender',
              type: 'address',
              value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
            },
            {
              name: 'amount0In',
              type: 'uint256',
              value: '240000000000000000000000000000'
            },
            {
              name: 'amount1In',
              type: 'uint256',
              value: '0'
            },
            {
              name: 'amount0Out',
              type: 'uint256',
              value: '0'
            },
            {
              name: 'amount1Out',
              type: 'uint256',
              value: '229010580386381830'
            },
            {
              name: 'to',
              type: 'address',
              value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
            }
          ],
          location: {
            instructionIndex: 5459
          },
          name: 'Swap',
          startIndex: 5459,
          topics: [
            '0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822',
            '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
            '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
          ]
        }
      ],
      rawInput:
        '0x022c0d9f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000032d9be8d5bc8006000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b00000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000',
      rawOutput: '0x',
      startIndex: 3507,
      storages: [
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 3638,
          mappingKeys: [],
          slot: '0x000000000000000000000000000000000000000000000000000000000000000c',
          startIndex: 3637,
          type: 'SLOAD',
          value:
            '0x0000000000000000000000000000000000000000000000000000000000000001'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 3646,
          mappingKeys: [],
          slot: '0x000000000000000000000000000000000000000000000000000000000000000c',
          startIndex: 3645,
          type: 'SSTORE',
          value:
            '0x0000000000000000000000000000000000000000000000000000000000000000'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 3668,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000008',
          startIndex: 3667,
          type: 'SLOAD',
          value:
            '0x647df6e30000000000023723fadaf09662260217ebe63a2743286801cae23364'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 3714,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000006',
          startIndex: 3713,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 3716,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000007',
          startIndex: 3715,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc2'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5181,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000008',
          startIndex: 5180,
          type: 'SLOAD',
          value:
            '0x647df6e30000000000023723fadaf09662260217ebe63a2743286801cae23364'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5270,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000009',
          startIndex: 5269,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000000062e9c7e4818536c2e0568d84'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5282,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000009',
          startIndex: 5281,
          type: 'SSTORE',
          value:
            '0x000000000000000000000000000000000000000062ec035968984a049636f5fc'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5329,
          mappingKeys: [],
          slot: '0x000000000000000000000000000000000000000000000000000000000000000a',
          startIndex: 5328,
          type: 'SLOAD',
          value:
            '0x000000000000000000004825f54106119da8b66e743847721974bb462bdcc044'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5341,
          mappingKeys: [],
          slot: '0x000000000000000000000000000000000000000000000000000000000000000a',
          startIndex: 5340,
          type: 'SSTORE',
          value:
            '0x000000000000000000004827f3876796e5d79a3a2c6e0cbc903bcb25a3ef2ecc'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5345,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000008',
          startIndex: 5344,
          type: 'SLOAD',
          value:
            '0x647df6e30000000000023723fadaf09662260217ebe63a2743286801cae23364'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5379,
          mappingKeys: [],
          slot: '0x0000000000000000000000000000000000000000000000000000000000000008',
          startIndex: 5378,
          type: 'SSTORE',
          value:
            '0x647df8ff00000000000233f65ef21ad9e220021af36192fd16a0a11b4ae23364'
        },
        {
          address: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
          endIndex: 5465,
          mappingKeys: [],
          slot: '0x000000000000000000000000000000000000000000000000000000000000000c',
          startIndex: 5464,
          type: 'SSTORE',
          value:
            '0x0000000000000000000000000000000000000000000000000000000000000001'
        }
      ],
      to: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      toContractName: '0xf8956e715b9aa5897c6e81ce50b4c7256f43df21',
      type: 'CALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      depth: 1,
      dynamicLogs: [],
      endIndex: 5679,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'balanceOf',
      gas: '0xd6b6',
      gasUsed: '0x216',
      inputs: [
        {
          name: 'account',
          type: 'address',
          value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
        }
      ],
      location: {
        instructionIndex: 5569
      },
      logs: [],
      rawInput:
        '0x70a08231000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      rawOutput:
        '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006',
      returnValue: [
        {
          name: '',
          type: 'uint256',
          value: '229010580386381830'
        }
      ],
      startIndex: 5569,
      storages: [
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 5658,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 5657,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006'
        }
      ],
      to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      type: 'STATICCALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      depth: 1,
      dynamicLogs: [],
      endIndex: 6219,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'balanceOf',
      gas: '0xce5c',
      gasUsed: '0x216',
      inputs: [
        {
          name: 'account',
          type: 'address',
          value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
        }
      ],
      location: {
        instructionIndex: 6109
      },
      logs: [],
      rawInput:
        '0x70a08231000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      rawOutput:
        '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006',
      returnValue: [
        {
          name: '',
          type: 'uint256',
          value: '229010580386381830'
        }
      ],
      startIndex: 6109,
      storages: [
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 6198,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 6197,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006'
        }
      ],
      to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      type: 'STATICCALL',
      value: '0x0'
    },
    {
      calls: [
        {
          calls: [],
          contractName: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
          depth: 2,
          dynamicLogs: [],
          endIndex: 6488,
          from: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          fromContractName: '',
          functionName: '0x',
          gas: '0x8fc',
          gasUsed: '0x3e',
          inputs: ['0x'],
          location: {
            instructionIndex: 6472
          },
          logs: [],
          rawInput: '0x',
          rawOutput: '0x',
          returnValue: '0x',
          startIndex: 6472,
          storages: [],
          to: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
          toContractName: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
          type: 'CALL',
          value: '0x32d9be8d5bc8006'
        }
      ],
      contractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      depth: 1,
      dynamicLogs: [],
      endIndex: 6525,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: 'withdraw',
      gas: '0xca0f',
      gasUsed: '0x23f2',
      inputs: [
        {
          name: 'wad',
          type: 'uint256',
          value: '229010580386381830'
        }
      ],
      location: {
        instructionIndex: 6329
      },
      logs: [
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          data: '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006',
          endIndex: 6521,
          events: [
            {
              name: 'src',
              type: 'address',
              value: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
            },
            {
              name: 'wad',
              type: 'uint256',
              value: '229010580386381830'
            }
          ],
          location: {
            instructionIndex: 6520
          },
          name: 'Withdrawal',
          startIndex: 6520,
          topics: [
            '0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65',
            '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b'
          ]
        }
      ],
      rawInput:
        '0x2e1a7d4d000000000000000000000000000000000000000000000000032d9be8d5bc8006',
      rawOutput: '0x',
      startIndex: 6329,
      storages: [
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 6411,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 6410,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006'
        },
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 6441,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 6440,
          type: 'SLOAD',
          value:
            '0x000000000000000000000000000000000000000000000000032d9be8d5bc8006'
        },
        {
          address: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
          endIndex: 6448,
          mappingKeys: [
            {
              baseSlot:
                '0x0000000000000000000000000000000000000000000000000000000000000003',
              key: '0x000000000000000000000000ef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
              keySlot:
                '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
              offset: 0,
              slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a'
            }
          ],
          slot: '0x29637e4e7746790fee278e174f4585f655f8f9eb93ccdfba31b482e7a484285a',
          startIndex: 6447,
          type: 'SSTORE',
          value:
            '0x0000000000000000000000000000000000000000000000000000000000000000'
        }
      ],
      to: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      toContractName: '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
      type: 'CALL',
      value: '0x0'
    },
    {
      calls: [],
      contractName: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d',
      depth: 1,
      dynamicLogs: [],
      endIndex: 6577,
      from: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      fromContractName: '',
      functionName: '0x',
      gas: '0x8bee',
      gasUsed: '0x0',
      inputs: ['0x'],
      location: {
        instructionIndex: 6577
      },
      logs: [],
      rawInput: '0x',
      rawOutput: '0x',
      returnValue: '0x',
      startIndex: 6577,
      storages: [],
      to: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d',
      toContractName: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d',
      type: 'CALL',
      value: '0x32d9be8d5bc8006'
    }
  ],
  contractName: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
  depth: 0,
  dynamicLogs: [],
  endIndex: 6615,
  from: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d',
  fromContractName: '',
  functionName: 'execute',
  gas: '0x31ae2',
  gasUsed: '0x2177b',
  inputs: [
    {
      name: 'commands',
      type: 'bytes',
      value: '0x080c'
    },
    {
      name: 'inputs',
      type: 'bytes[]',
      value: [
        '0x00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000003077b58d5d378391980000000000000000000000000000000000000000000000000000000032b2ced3e40e9d100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc2',
        '0x0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000032b2ced3e40e9d1'
      ]
    },
    {
      name: 'deadline',
      type: 'uint256',
      value: '1685979119'
    }
  ],
  location: {
    instructionIndex: 0
  },
  logs: [],
  rawInput:
    '0x3593564c000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000647dffef0000000000000000000000000000000000000000000000000000000000000002080c000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000160000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000003077b58d5d378391980000000000000000000000000000000000000000000000000000000032b2ced3e40e9d100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc200000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000032b2ced3e40e9d1',
  rawOutput: '0x',
  refund: '0x765c',
  startIndex: 0,
  storages: [
    {
      address: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      endIndex: 162,
      mappingKeys: [],
      slot: '0x0000000000000000000000000000000000000000000000000000000000000001',
      startIndex: 161,
      type: 'SLOAD',
      value:
        '0x0000000000000000000000000000000000000000000000000000000000000001'
    },
    {
      address: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      endIndex: 168,
      mappingKeys: [],
      slot: '0x0000000000000000000000000000000000000000000000000000000000000001',
      startIndex: 167,
      type: 'SSTORE',
      value:
        '0x0000000000000000000000000000000000000000000000000000000000000002'
    },
    {
      address: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
      endIndex: 6612,
      mappingKeys: [],
      slot: '0x0000000000000000000000000000000000000000000000000000000000000001',
      startIndex: 6611,
      type: 'SSTORE',
      value:
        '0x0000000000000000000000000000000000000000000000000000000000000001'
    }
  ],
  to: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
  toContractName: '',
  type: 'CALL',
  value: '0x0'
}

const block = {
  baseFeePerGas: '0xe1e06e3be',
  difficulty: '0x0',
  extraData: '0x6265617665726275696c642e6f7267',
  gasLimit: '0x1c9c380',
  gasUsed: '0xa3a8f4',
  hash: '0xd6931aa5c9863f6011255db4ae03f72f72a787a0a93bd12cf6a0c3eba0c80401',
  logsBloom:
    '0x0da7103a23804c880201320582402e02d40104880610a1160801060fa32880564869986040501520c708204c0211815d96898932ce022e050e0210080c7de012cc11ff224c208a082801462d9a20546e6db0104181461b404a0a0876986a09800a021120420ea4000d4981b108520c5c27c09253098885ac80ba631ac26a4055275006401b097024c06f00400120089653013c9d65ae9088e04098444014900c0268618811c9e2c0a4404bb422888ae82532ea293060de08617c28e574000141de702622006306fc88208cc190305419c049640fa689207c432121f602206c17ac51a09812068382858585a800420902183a831cc1e8a14180566800a2764c0c',
  miner: '0x388c818ca8b9251b393131c08a736a67ccb19297',
  mixHash: '0x9951b939eb5f87082f6f462a0bc1355a3840261e82659883532a658c1ad37fa0',
  nonce: '0x0000000000000000',
  number: '0x109bba0',
  parentHash:
    '0x19a0a858b3f0ec6df0fa0f768ab2e0f969080e38a2bf386a771e61ba67e126d7',
  receiptsRoot:
    '0x51b8ea70448011f67637442988440ad9d97ea8b8799af18f1ea0c4e436dce19f',
  sha3Uncles:
    '0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347',
  size: '0xafe9',
  stateRoot:
    '0x096f15f64754cc4f5c30d351d0cae4eff31804b999780f21e3017512505fcc42',
  timestamp: '0x647df8ff',
  transactions: [
    '0xc6176f1c6f764c6bbfce03b7a7da8b99d4f4989a4eb0235929ca3619c4d4f41c',
    '0x60b81fa02a1a99ab02e186241d992f6210c54b3a004add39f0614f1b690ecc1d',
    '0x50ea66fc5d7161bf8baa6f811d357736ac680f4fb97989db09aa3b83d7b92c34',
    '0x289ed1e780f93187418bcd97275a9fb7e97f3c8ef056b2361da5ddec1a202f44',
    '0x0bd4a218020bdf2cd48274c6eddc646d7b6dcabb2ec8210d54bec597a51ac72f',
    '0x01e42fd74818cf3e8c94f49dd9aa627d4023f782bcaf2d44647a888418c154c6',
    '0x64d30d27ab0e1ffaf3089a62564b19103734ecfdd1dedd83dde7d7b3fd16e0ad',
    '0x859dfa0fca0063940a367321861daaf50b1eeda3ece84274bd69c338be708988',
    '0xdd188beb8a461afdb3636ffcdaa0a6f74b9eea54a90c57a3b3969cc5f5c82b34',
    '0x844747f7f808d8e5003ebb8f54e62c3ce0dc2809263960fa58a08055b975a5e1',
    '0xe3fdc02d07bb81e61cfffb15e48a25f731fbd3f2e5b4567fb563cf3ea6e57ee4',
    '0x222193776bf5da352bdd3a925367c59d5dabb1cd549b0aff69467ebbf286ffd9',
    '0x8a24ab8ae6a8edcf1a4ee941bda1f549a012bbb8ed46d00696dab5daa4690d5a',
    '0x1a4e07d50e279107dbce626bebee6807212375e3cee06851e9b57695f3c25641',
    '0xca10e0a62c92569488fca948d2f538f675d5a8d1db84e66e0eb4ccc12a650e33',
    '0xcd600df2659a6892fe910e42c19d5d91e88d94974398c479bedd4cbf143b7426',
    '0x003f7f68117278079b9805918a64806d8aa615abf6a7620e055c976efc506c6e',
    '0x46b273ac3dc18d3aaf91f987485679a271e4b380a619698917fea80c7fecdcd3',
    '0x045439bbb77c0ce65629c9ab349449605aed8d6266e70028ec6dd38361f5cd37',
    '0x38b678407c2067174c75a585eb2e8fb75b607d3f69218d0b2ef5945e848d45b0',
    '0x750b9c7a25f0427776d54b6b6526ff8167359b09e3d82c866273259e6f20293e',
    '0xea53f589c44f56d0053dbe6ca6c02b8abc2a13fbb30295e71ac8ef782d458b65',
    '0x40463307f6b4cdeb5a4bafc095adf0e2ef09b17d13fe5fda1a35bf10570680b8',
    '0x9e3816c7ce6f237332dc70a88cbe9e3e296c651c8aae6c778126dab95ef60b84',
    '0x4f9a992767f060b2ac704bb7837255950232dcb8c6abae92c55d9fa6486212e7',
    '0xc0d1b9a45188ee9899f68ac122422bcf8fd6581ed876399c42db66b9a7502709',
    '0x0d516cbb2eb8f39ce6caf777f95dca2b7f352f71bf74c96db5f5bba4ff37d196',
    '0xb63eec113e2aaa927a655e7baca5a67b63ee666c3d1fc85d10cc8d6c9334573f',
    '0x8aa7b214c22a4e95d30505b7f65f4c39273a0803099f4774d91435ceb667839b',
    '0xa7aab5d2bfe4b05c43ffbcad0943c39a34fa200cdafa1cda0bb0d50caf542509',
    '0xdd54984692845b9a03578bbc4a2208da82a0a8cb2bd3b748d8a9919c4f2632c7',
    '0xbafc490848355ec28d2f5453eba2a4ee49b1fc3f3b9acd5fa2903dd25f8cb518',
    '0x65081480e9dc6c372523f625e880944f9feef400ed096a3783e8cc8f5d692123',
    '0x5b79bc0c9f033ea29aa86eccfcf7860fa83fe5952ea54d68b062a0e8e691990f',
    '0xdc024671ba11cbb4683a47ae117ffc35b915c5233eb2c07b8fa62becbdcb9a2e',
    '0x86acc71c2bed1f530218b23595c8a2ada543ebfe44437dd8057d2d7e769dc46f',
    '0xa43d2d2e564e9e22528f7c0b64f39998a1430a6958fad4697b83540e20d169b6',
    '0xdcf3f7b8b6c754344d1d8b599113f2e659864af42dff535186db97bd5c1745ab',
    '0x598ddac49c42c0a72e8bcd534675d20941efc8ad0aae6394acce46b636ec26b8',
    '0x26b53479c3a6be49d41b80c776a8061ff369aeccb3e44f34b16c533c7ee55500',
    '0x94d6ac018ae468f5cebbb79ed61d695a1d3d7cc651681aca46709749aae55efb',
    '0x31f3176c94504ed57a3de42a2f0d27936bc8d3b11cbf641deaf57497c4e81c1a',
    '0xd6c34f630a96bfbf4c564826893dd8fc4257d17ac8d608ff3eb7cbfb677c0f9b',
    '0x3eb4583c01e529948cdbe125e08b844b8e51bff010fa45cdd218bd87318e048d',
    '0x43e7b191ff1b44c0a67ecac629a360602a413ecc90e6e3b3fbaf7bc4352b825a',
    '0x8587f308f20678461dbc9130083a8cd3b1ff8ca6dadd67d8617b3fd451cecc96',
    '0xc9851a14754090318ea2e4e7208426648e294f1f22c038a2d51c7207b7e270d2',
    '0x895682c3cea546566b8cadc21bf86216cf61861fa570efbd3085f769e0f934f4',
    '0xe1ce82c8fce76d06502d27be7f3f8ec75ca659eb2b52077e49bf47d0e2124cf8',
    '0x985aa5b1439db6a148e9c892365e7ba239d2ea6008008b95709d04466817cf2d',
    '0x3460ebe1f868bc016482d325787e7471f249d350d56da7e2676d333aa51c9404',
    '0x125539175f85d968a7c0d675b7db1b1bfdb557a680defd7b8fa3e74048923838',
    '0x1170ac08ac1f8162bb5f9fcb634fa3a93500cb04690df38402e9392128967c15',
    '0x8153bad4ccd9354ee8aa5d4d0702fddb9ef397ae1a5785a53da5558eee0c44ab',
    '0xcf17960fc4be7f2e61c5383e2a995061680f6442f7cce616d4009f68ee95961c',
    '0x7c785df92b1e16ee3d029a4b18da49ac9a3fa6b41efd1240c6fbbcca93f36969',
    '0xd041479009c8585e4d684afe8b0e85801d0dc37b9daf3a63732610bfc7f75f6d',
    '0x04dea77452fcbc42362f8242c91a6736b238c4411775f8b579cc0640d122d012',
    '0xcf20c76d02e0527786784bd02fb744ef1f08dd75828773e71bc23a1344866d97',
    '0x7819733635a176b32d348aa4eabdc7ea1c0301fbd45734af7b1ce1cd1714b060',
    '0x10afa6621cf5ba2023d592d0115041f8fe9f01d46e74b99f9c5fb86658869389',
    '0xdf8ba8bbe3a76f0b81b145f4b62a11b7e93177be0077cf71f90044731137b378',
    '0x342131635e256b977b749f9eb1dd21836fd8503f6e63020f610d36e8c9701899',
    '0x075eb79b071c1051b53b6d229b3f05a2b7ad6dcc8b8d7edcd7573055f5a16ab7',
    '0xb337d1d84ddfd72b2f5571492f448aa4e8451f32576010444c2d90f226015617',
    '0x11f46ddfeaec45a8df00240100f30a792f002e03463fe59f930982ae043522b0',
    '0x70badd2b9ed5854ae02017d5dbf899da3c663c5256e8ea7474cac25e723ddf29',
    '0xefa9cc046828588ec593b430e267e436f5e2cb1120d248dbf126528968db0bc4',
    '0x1a5e356cd9971916cb3c6bff0ae15e8c8258cf2aaa22ef8018a22ac31fa4626a',
    '0x8d04cc5588f29d64bf3cbf8555ea3761e55392daf0998faeddc79ec9b7266ac6',
    '0x025ec42355da1f12314d95cc6f19fcb52162107a04a331b1d8a44636b3b98d00',
    '0x20a859b52714230cbbef0b0d85d077d8693828254b9c662fe6fa4e21cd69d12b',
    '0x71393ba3de743a75871d19aafd955745fd6f9b5dae2256e73b4461d032b99437',
    '0x8881ef257f67ea92e12475658dcfcebef69a83092e788b254afc926594310d4d',
    '0xa525b6d4a9472d47bad208aa04a4d6929c0261eefd542c74915421e02064a771',
    '0x0388aa1b3c5dc8425b5cc4f17357bbbff0d57d904b048e92b58c42f463d6a5f5',
    '0xee1479686fba1047f8e297baaf0c138aa60dbc42c1fe1dc2a62e44bee07549fa',
    '0xbec1c6031817dad1541eb260762931f5ddff5bb75bc4bfdcc4d45db5c63f3f38',
    '0x2f0b3d3e9e5ce2e5cd4e337a45d7e1b7d098557233a69d89ce995e1dcfd9848b',
    '0x6fec73e8194748f9aa624071893b9a65ec025c60e0ead9079a756cb90b8be8b2',
    '0x31c0a08d01f2d36b155ac6a25e33078861dadce8fbdd8013c90d1101bfb4cd85',
    '0x89464031d889a760aadda70fa95616f2adea8071bd9a013aa164d70504b0ac7f',
    '0xd7899888eda7c9cbe6777463718a95990ad67eccd81451930f37e8b5779dafa2',
    '0x84049d2bb98b46175a68b19896171c20a1818fa330cf0ea3f71018fd7768da0d',
    '0x85cde9e5e66ca777f60f2efc4711d1c8a6fa84ea16652d532bcdedd2d6317abd',
    '0x9a34d7f661e255bf73409c0166f01e0c665469f0727a23f5e45e142e8386f864',
    '0x30793d1aae619610ff74cf28fe767a436541be6a9331ef5ca87fc54ef06f7c93',
    '0x65cc1f7f38fe40a907c41f7b0459e66efc4f1396b311ad7d4b5f3a321592eb31',
    '0x6d44799ee8b479f68617a149f135ee3366f41266b73b3bf6d488ec5c34343aad',
    '0xa5f22dc77f70388a480f1e2b5706196d90b1a4080efd1a4d951f24c799add010',
    '0x6c4b144e7a80f955a7e300e3ec3d9f2aaa9a0413c5d46885f77d88660b646a59',
    '0x07e372218c4894fbd33705175e3b3b01eaff87eaf1b659d7170bdee5ca6e8117',
    '0x4b815b85fdf2a7c6d643416b371e33dd9b4a627d0a9327f1c4af392194573d0f',
    '0x8bdc5c69572f321e934236d90199a2475bb52227356bd7f281568857dcde9b98',
    '0xf20b79139c42e38c3d7fbd78386daf893555461c3b29c65fab90defe5268eca9',
    '0x4ce777c0ecd1d1f718747cfbd6e08042b8577576ec67a5bd9ccbcead7d0a1e25',
    '0xeab69dc6748bf07afcc187d2fad0143ee786eb8ceb5a0ba4845006356f6c0681',
    '0xcae6f554639be31becd00904d20d0e6d90030c9f00683688ec3fdeb9c87c196d',
    '0x2e755fb0903637690b5ff0a5d3ec0120b83b9115f355a0b2613afbb9b72f4f59',
    '0x0ccd74d08adf53aaaf961b5fdedb82a23c4d947175991aea96bba5453fb6b852',
    '0xb3dae1218b9786ff90016c266aa8b877fc2a07a89f4fceb64f77790ed85f86a3',
    '0xb22db58db1ac34d40420322c5f08c2c128c2150db8b59dc28f1edeadcc56d5cb',
    '0xed42a1ff99a336f663c120590192de7df8e67cd8c318ebda129bb3a5b3224acd',
    '0x3c695b1b3cb7ff3225b6320c19b4015da202fe4594d32a4fcf30ae83ab435c44',
    '0x1c76f8c33f4ce88b76b18722219648cc4ab01c61d672b8e793170b1f9e139c62',
    '0x12833e1236f494134e1b9b0ede40eb5922a17388eb48bcda1982d2b61af58e0b',
    '0x01b59be1262b206501afdc584a820b6fe9646825cc166b70eaff9066eb1d1a4c',
    '0xec057e70baf384f97de33efea6704f80207646115578825dec58d850a7b022f0',
    '0x441533181127a0997ba585bbd4e652575d92b9132027b7db13ade9004f734072',
    '0x8a750c5b1a65b7a1580dba7444fc0399545405204fcf7d51fb989b9f964c6655',
    '0x14a4718899dfaa83f80ad6826cd4228d9bed4a080594da6a8dc4f03f81bb9445',
    '0xd02c1c64b162dbf9295f50a0963029a194aa50a797f0f2384c95d02bf7ef0a84',
    '0x6dcacb2b7b7ad295ac61a824466c7f2c3302db58e79988adf00dad911e7ad026',
    '0xd467a640ce24ace0126e834d5ff21f6599f3af968b81c78da096bd60f5d3bce0',
    '0x88dc94d04a35f7afdd3e25bbcebc2412bcf1aae9c1f45256c01a64f18289e2d9',
    '0x438696375cc40b391c282fb66cf7942327803e3cbfc49d72bc4d93c055772f4e',
    '0x1fc2437d3e2affc2b90a91b12e20ccbbc388b1f4410a7f02fcb37d8fe60c32f1',
    '0xd9102321221d53e9386026807ced2d79d14402da8ca1439fb20a330a124cf5d8',
    '0xffc24272c5252071840045bdfef164d8e54c7d13f8cfead0fbde229e6476b1be',
    '0xb34d73cb7f4027ad83e137349dfa1af0752871220e493bf56d3250f46a12db80',
    '0x169bb382ce54a8ef7ceb22aeb2e683eb4a25c5e570561a862c87cf880ef1df6f',
    '0xb2f506542eee56df9557703488348ddfc11da0b78a9e898856ef680877ec50d9',
    '0xe6ad98cdeb29a79ea80e98e9966f2c861471159c4b758b874bae19f0647c8d94',
    '0x91c61e944f4921d1a0c5b9624fdc6c6a2049e49a090d7a631d76bfa348efdf7d',
    '0x6803cd308d7714d0b5f2570b7220f8215a40cb64022e9ddfc5a050cadf14e810',
    '0x3c01d8bbd8fcffbec511e9b5c1114635e3e3798024972d963237cf0225220b8c',
    '0xb76dfb922f4d7295d89b1928fb68aee0af25f8abf9f78c7e5d87acd60305aa93',
    '0x45b0195bb85ab43854d3b57b526eaac8024f9cb9cb032c366d92242289d04da8',
    '0xfee85429c1232b4f30f2af4d243d43efe8f4241907eae4787f40cf0cc90cc2de',
    '0x2096ee89df427684ee267c43a2c7c84454fe57ce453587702b5e21c9cda701bb',
    '0xfd9b2dca389837430e68492c2c4da0db79175c33b531fdf20bfc225c775e1eb3',
    '0x1e6d95587c5950e5b8117346525664913b5bec1363fd628e4cf71e56603448d2',
    '0x4e49decd1cd630bea31d9090894d416884bcf30a8fc8dcaa7e84f6d48627f669',
    '0x209e911bfa7b5f39173234518ea988fa10e89a8478e1f49a48adddfb6214f571',
    '0xf3b3d73f97b0e924d0eab0ae5e7d2d45e169cbf08513b412fde4376d4ca48a74',
    '0x5f3fc6c6a2813e8eb0814f7ec82a145860c0690ea800d2322db69e224140de9c',
    '0x70ec70bf2053460b0a7ff1eeb4d4398c4f5ab99de15b973d929bd2107a71a7cd',
    '0x432c4c011eb95d6270098258f770c0f0c4b1a26303ea1842a03a3573dad51db2',
    '0xfa794e701760fdd5be3a099dcae2eec7d087a792d45102a9d572800552067db4',
    '0x4fe752498e296ef3446c9942aeed6ee06e5326f7388182582afc09d443c73fb1',
    '0xfbedf16897816b63b615c66d605f7f905b709ca45b100ad229fbaf12dcd6a113',
    '0xf28dad502d6f3dd9d804aa6723bff4fd4cf2bc9b5ec5de10403ed7d4ecbda9eb',
    '0x992bb871b98bd9b9eccbc3225cab443834fb31538502fe21c241f1089d323a6a',
    '0x11f21e93cc006e8445fa5a2c924f663038d6a531e3d58e0adb45a7c0c152ce53',
    '0x806f974bf127a4870a4229104885db60e89a226c01976a70e67eea66fc9061ef',
    '0xda1f8c7e6e96e370bfa914cc1f720a6f2fad20665aed86265dece914ad348f34',
    '0xed31fcb7f948e9c21648de50bfa0846932e01d2b629e8f1e0a996a79eb6b505e',
    '0x2d703bba40efde3adb292bd9f110f436e540bcf704845a521ae3dbc0523e831b',
    '0xd08db206e02622f36098c20a20200318fc83c2fbc8f8fc80d229cecde067bb02',
    '0x4e319fbbfcfdb2b0c8cbfbcab52523dcf91b8f05615f86615ffbaf21ba4cd5a8',
    '0x4acbab8e42fae49f31f6de938458bef02a4209dca62214b4b7b28efba3099ebf',
    '0xa122ca9eed8187af254381cfdd06d39444fd42894838281eaed58e990d97385a',
    '0xdd3ede971f0eeee86263dcb992bdca9b90a28c0130deccd39abddb9cfd0d16fa',
    '0xd171193a63bf37afdbdb00a9dbedb11a79ae34652e5f441b31ab2d8fe6282f99',
    '0xf4de5a588886b744462b47f045cd7a27313540a9fb97ef3f10b5d0ce48de3357',
    '0xcd90cf11960938c71f93aebc600a29cb59a5dec9a33ee412b4062f4ad757f7ef',
    '0xe9daede86c36864e5ff54a17e49e997c4de4c123bd0c8374f064538e4489239a'
  ],
  transactionsRoot:
    '0x1c08b36848038202edf397831f51cbf6806a2516564b7bf27fb9d548f987737b',
  uncles: [],
  withdrawals: [
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcde42c',
      index: '0x5cd0ec',
      validatorIndex: '0x2e2c3'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcdb87d',
      index: '0x5cd0ed',
      validatorIndex: '0x2e2c4'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xce54b6',
      index: '0x5cd0ee',
      validatorIndex: '0x2e2c5'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcf3833',
      index: '0x5cd0ef',
      validatorIndex: '0x2e2c6'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcdef22',
      index: '0x5cd0f0',
      validatorIndex: '0x2e2c7'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xce6277',
      index: '0x5cd0f1',
      validatorIndex: '0x2e2c8'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcef777',
      index: '0x5cd0f2',
      validatorIndex: '0x2e2c9'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcebfdd',
      index: '0x5cd0f3',
      validatorIndex: '0x2e2ca'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xce01e9',
      index: '0x5cd0f4',
      validatorIndex: '0x2e2cb'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcee2dc',
      index: '0x5cd0f5',
      validatorIndex: '0x2e2cc'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcd6ba4',
      index: '0x5cd0f6',
      validatorIndex: '0x2e2cd'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcec6ad',
      index: '0x5cd0f7',
      validatorIndex: '0x2e2ce'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcd9f99',
      index: '0x5cd0f8',
      validatorIndex: '0x2e2cf'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcd4a29',
      index: '0x5cd0f9',
      validatorIndex: '0x2e2d0'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcf04e6',
      index: '0x5cd0fa',
      validatorIndex: '0x2e2d1'
    },
    {
      address: '0xa8c62111e4652b07110a0fc81816303c42632f64',
      amount: '0xcfa3cc',
      index: '0x5cd0fb',
      validatorIndex: '0x2e2d2'
    }
  ],
  withdrawalsRoot:
    '0xe57213bd5b4c4155fef256c8c78ee3ee40139069d6be84f203bf32e80cef3f12'
}

const transaction = {
  blockNumber: '0x109bba0',
  blockHash:
    '0xd6931aa5c9863f6011255db4ae03f72f72a787a0a93bd12cf6a0c3eba0c80401',
  transactionIndex: '0x61',
  hash: '0xcae6f554639be31becd00904d20d0e6d90030c9f00683688ec3fdeb9c87c196d',
  chainId: '0x1',
  type: '0x2',
  from: '0x5e8bb488e85ea732e17150862b1acfc213a7c13d',
  to: '0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b',
  input:
    '0x3593564c000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000647dffef0000000000000000000000000000000000000000000000000000000000000002080c000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000160000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000003077b58d5d378391980000000000000000000000000000000000000000000000000000000032b2ced3e40e9d100000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000082646b22a3960da69ef7a778c16dd6fb85dd999000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc200000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000032b2ced3e40e9d1',
  value: '0x0',
  nonce: '0x455',
  gas: '0x31ae2',
  gasPrice: '0xe59a1adbe',
  maxFeePerGas: '0x11b4a4f968',
  maxPriorityFeePerGas: '0x3b9aca00',
  accessList: []
}

async function priceFetcher(params: any) {
  const address = params.coinId.address.address as string
  const values: Record<string, number> = {
    '0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2': 1814.7234371556767,
    '0x0000000000000000000000000000000000000000': 1813.6816549003997
  }
  return {
    price: values[address] || 0
  }
}

export const Default = () => {
  return (
    <div className="text-sm">
      <PriceFetcherContext.Provider value={priceFetcher}>
        <SvgFolderContext.Provider value="https://app.sentio.xyz">
          <ChainIdContext.Provider value={'1'}>
            <CallTracesContext.Provider
              value={{
                data: callTraceData,
                loading: false
              }}
            >
              <BalanceChanges
                transaction={transaction as any}
                block={block as any}
              />
            </CallTracesContext.Provider>
          </ChainIdContext.Provider>
        </SvgFolderContext.Provider>
      </PriceFetcherContext.Provider>
    </div>
  )
}
