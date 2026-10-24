import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { flattenModelUsageData } from './glm-usage'

// 样本对齐智谱 token-usage/usage-detail 接口的真实响应结构：
// 时间轴与模型明细嵌套在 modelUsage 内（xTime 为驼峰），summary 为顶层比率/Token 汇总。
const sampleData = {
  granularity: 'DAY',
  timezone: 'Asia/Shanghai',
  summary: {
    cacheHitRate: { value: '0.9441', trend: '-0.0104' },
    offPeakUsageRate: { value: '0.9078', trend: '-0.0755' },
    totalTokens: { value: '2157857407', trend: '-0.0889' },
    averageDailyTokens: { value: '71928580.23', trend: '-0.0889' },
  },
  modelUsage: {
    totalUsage: {
      totalTokens: 2157857407,
    },
    modelDataList: [
      {
        modelCode: 'glm-5.3',
        modelName: 'GLM-5.3',
        sortOrder: 1,
        uncachedInputTokensUsage: [7369882],
        cachedInputTokensUsage: [213324608],
        inputTokensUsage: [220694490],
        outputTokensUsage: [1413699],
        totalTokensUsage: [222108189],
      },
      {
        modelCode: 'glm-5.3-flash',
        modelName: 'GLM-5.3-Flash',
        sortOrder: 2,
        uncachedInputTokensUsage: [5671879],
        cachedInputTokensUsage: [74319424],
        inputTokensUsage: [79991303],
        outputTokensUsage: [251234],
        totalTokensUsage: [80242537],
      },
    ],
    modelSummaryList: [
      {
        modelCode: 'glm-5.3',
        modelName: 'GLM-5.3',
        sortOrder: 1,
        totalTokens: 1000200733,
      },
      {
        modelCode: 'glm-5.3-flash',
        modelName: 'GLM-5.3-Flash',
        sortOrder: 2,
        totalTokens: 1157656674,
      },
    ],
    xTime: ['2026-09-25'],
  },
}

describe('flattenModelUsageData', () => {
  test('reads the time axis from modelUsage.xTime', () => {
    const result = flattenModelUsageData(sampleData)
    assert.deepEqual(result.times, ['2026-09-25'])
  })

  test('builds one stacked series per model and token kind', () => {
    const result = flattenModelUsageData(sampleData)
    // 2 个模型 × 缓存/未缓存/输出 3 个系列 × 1 天
    assert.equal(result.values.length, 6)
    const numericSeries = result.values
      .map((point) => point.value)
      .sort((a, b) => a - b)
    assert.deepEqual(
      numericSeries,
      [251234, 1413699, 5671879, 7369882, 74319424, 213324608]
    )
    assert.equal(result.fields.length, 6)
  })

  test('aggregates model summary and total tokens', () => {
    const result = flattenModelUsageData(sampleData)
    assert.equal(result.total, 2157857407)
    assert.deepEqual(result.summary, [
      { modelName: 'GLM-5.3', totalTokens: 1000200733 },
      { modelName: 'GLM-5.3-Flash', totalTokens: 1157656674 },
    ])
  })

  test('formats summary stats with ratio / compact numbers and trend', () => {
    const result = flattenModelUsageData(sampleData)
    assert.equal(result.stats.length, 4)
    assert.equal(result.stats[0].display, '94.4%')
    assert.equal(result.stats[0].trend, '-1.0%')
    assert.equal(result.stats[2].display, '2.2B')
    assert.equal(result.stats[3].display, '71.9M')
    assert.equal(result.stats[3].trend, '-8.9%')
  })

  test('returns an empty payload when modelUsage.xTime is missing', () => {
    assert.deepEqual(flattenModelUsageData({}), {
      values: [],
      total: 0,
      summary: [],
      stats: [],
      fields: [],
      times: [],
    })
  })
})
