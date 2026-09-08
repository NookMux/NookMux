/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  BILLING_TOKEN_FIELDS,
  buildTokenBreakdownGroups,
  buildTokenTooltipRows,
  getPriceSnapshotComponentLabelKey,
  getPriceSnapshotComponentQuantity,
  formatPriceSnapshotUnitPrice,
  parseBillingDetails,
  resolveDisplayTokens,
} from './billing-details'

function completeBillingDetailsJson(
  overrides: Partial<Record<(typeof BILLING_TOKEN_FIELDS)[number], number>> = {}
) {
  const values = Object.fromEntries(
    BILLING_TOKEN_FIELDS.map((field) => [field, overrides[field] ?? 0])
  )
  return JSON.stringify({
    schema_version: 1,
    tokens: {
      input: {
        text_input: values.text_input,
        image_input: values.image_input,
        audio_input: values.audio_input,
        video_input: values.video_input,
        document_input: values.document_input,
      },
      output: {
        text_output: values.text_output,
        audio_output: values.audio_output,
        image_output: values.image_output,
        reasoning_output: values.reasoning_output,
        accepted_prediction: values.accepted_prediction,
        rejected_prediction: values.rejected_prediction,
      },
      cache: {
        read_cache: values.read_cache,
        write_cache: values.write_cache,
        write_cache_5m: values.write_cache_5m,
        write_cache_1h: values.write_cache_1h,
      },
    },
  })
}

describe('parseBillingDetails', () => {
  test('empty value is missing, not legacy', () => {
    assert.deepEqual(parseBillingDetails(null), { status: 'missing' })
    assert.deepEqual(parseBillingDetails(''), { status: 'missing' })
  })

  test('schema v1 keeps official token dimensions', () => {
    const raw = completeBillingDetailsJson({
      text_input: 60,
      audio_input: 20,
      reasoning_output: 3,
      read_cache: 40,
      write_cache: 5,
      write_cache_5m: 5,
    })
    const parsed = parseBillingDetails(raw)

    assert.equal(parsed.status, 'valid')
    assert.ok(parsed.status === 'valid')
    assert.equal(parsed.tokens.text_input, 60)
    assert.equal(parsed.tokens.audio_input, 20)
    assert.equal(parsed.tokens.reasoning_output, 3)
    assert.equal(parsed.tokens.read_cache, 40)
    assert.equal(parsed.tokens.write_cache, 5)
    assert.equal(parsed.tokens.write_cache_5m, 5)
    assert.equal(parsed.tokens.write_cache_1h, 0)
  })

  test('explicit official zero and every schema dimension survive parsing', () => {
    const raw =
      '{"schema_version":1,"tokens":{"input":{"text_input":0,"image_input":1,"audio_input":2,"video_input":3,"document_input":4},"output":{"text_output":5,"audio_output":6,"image_output":7,"reasoning_output":8,"accepted_prediction":9,"rejected_prediction":10},"cache":{"read_cache":11,"write_cache":16,"write_cache_5m":7,"write_cache_1h":9}}}'
    const parsed = parseBillingDetails(raw)

    assert.equal(parsed.status, 'valid')
    assert.ok(parsed.status === 'valid')
    assert.equal(parsed.tokens.text_input, 0)
    assert.equal(parsed.tokens.image_input, 1)
    assert.equal(parsed.tokens.video_input, 3)
    assert.equal(parsed.tokens.document_input, 4)
    assert.equal(parsed.tokens.accepted_prediction, 9)
    assert.equal(parsed.tokens.rejected_prediction, 10)
    assert.equal(parsed.tokens.write_cache_1h, 9)
  })

  test('rejects omitted and null token fields', () => {
    const omitted = parseBillingDetails(
      '{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{}}}'
    )
    const explicitNull = parseBillingDetails(
      '{"schema_version":1,"tokens":{"input":{"text_input":null},"output":{"text_output":3},"cache":{"read_cache":null,"write_cache":12,"write_cache_5m":7,"write_cache_1h":null}}}'
    )

    assert.equal(omitted.status, 'invalid')
    assert.equal(explicitNull.status, 'invalid')
    assert.ok(omitted.status === 'invalid')
    assert.ok(explicitNull.status === 'invalid')
    assert.equal(omitted.code, 'invalid_fields')
    assert.equal(explicitNull.code, 'invalid_fields')
  })

  test('rejects malformed JSON, unknown version and unknown fields', () => {
    assert.deepEqual(parseBillingDetails('{bad'), {
      status: 'invalid',
      code: 'malformed_json',
      errorKey: 'usageLogs.errors.billingDetails.malformed_json',
    })
    assert.deepEqual(parseBillingDetails('{"schema_version":2,"tokens":{}}'), {
      status: 'invalid',
      code: 'unknown_version',
      errorKey: 'usageLogs.errors.billingDetails.unknown_version',
    })
    const parsed = parseBillingDetails(
      '{"schema_version":1,"tokens":{"input":{"input_tokens":10},"output":{},"cache":{}}}'
    )

    assert.equal(parsed.status, 'invalid')
    assert.ok(parsed.status === 'invalid')
    assert.equal(parsed.code, 'invalid_fields')
  })

  test('rejects unsafe integers', () => {
    const parsed = parseBillingDetails(
      '{"schema_version":1,"tokens":{"input":{"text_input":9007199254740993},"output":{},"cache":{}}}'
    )

    assert.equal(parsed.status, 'invalid')
    assert.ok(parsed.status === 'invalid')
    assert.equal(parsed.code, 'invalid_fields')
  })

  test('rejects negative or fractional token values', () => {
    for (const textInput of [-1, 1.5]) {
      const parsed = parseBillingDetails(
        JSON.stringify({
          schema_version: 1,
          tokens: { input: { text_input: textInput }, output: {}, cache: {} },
        })
      )

      assert.equal(parsed.status, 'invalid')
      assert.ok(parsed.status === 'invalid')
      assert.equal(parsed.code, 'invalid_fields')
    }
  })

  test('rejects split cache without total or exceeding total', () => {
    const missingTotal = parseBillingDetails(
      '{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{"write_cache_5m":5}}}'
    )
    assert.equal(missingTotal.status, 'invalid')
    assert.equal(missingTotal.code, 'invalid_fields')

    const exceedingTotal = parseBillingDetails(
      completeBillingDetailsJson({ write_cache: 4, write_cache_5m: 5 })
    )
    assert.equal(exceedingTotal.status, 'invalid')
    assert.equal(exceedingTotal.code, 'invalid_cache_splits')
  })
})

describe('resolveDisplayTokens', () => {
  test('legal all-zero payload remains meaningful', () => {
    const empty = parseBillingDetails(completeBillingDetailsJson())
    assert.equal(empty.status, 'valid')
    assert.equal(resolveDisplayTokens(empty).hasValues, true)
    assert.equal(resolveDisplayTokens(empty).input, 0)
  })

  test('missing details never derive tokens from aggregate columns', () => {
    const tokens = resolveDisplayTokens(parseBillingDetails(null))

    assert.equal(tokens.input, null)
    assert.equal(tokens.output, null)
    assert.equal(tokens.cacheRead, null)
    assert.equal(tokens.cacheWrite, null)
    assert.equal(tokens.hasValues, false)
  })

  test('valid details do not derive tokens from aggregate columns', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 12,
        text_output: 7,
        reasoning_output: 3,
        read_cache: 4,
      })
    )
    const tokens = resolveDisplayTokens(billing)

    assert.equal(tokens.input, 12)
    assert.equal(tokens.output, 7)
    assert.equal(tokens.reasoningOutput, 3)
    assert.equal(tokens.cacheRead, 4)
  })

  test('valid cache splits expose their unallocated remainder', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        write_cache: 12,
        write_cache_5m: 7,
        write_cache_1h: 3,
      })
    )
    const tokens = resolveDisplayTokens(billing)

    assert.equal(tokens.cacheWrite, 12)
    assert.equal(tokens.cacheWrite5m, 7)
    assert.equal(tokens.cacheWrite1h, 3)
    assert.equal(tokens.cacheWriteUnallocated, 2)
  })

  test('invalid details never leak aggregate fallback', () => {
    const tokens = resolveDisplayTokens(parseBillingDetails('{bad}'))

    assert.equal(tokens.input, null)
    assert.equal(tokens.output, null)
    assert.equal(tokens.cacheRead, null)
    assert.equal(tokens.cacheWrite, null)
    assert.equal(tokens.hasValues, false)
    assert.deepEqual(buildTokenTooltipRows(tokens), [])
  })
})

describe('buildTokenTooltipRows', () => {
  test('reuses resolved official dimensions, explicit zeros and unallocated cache', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 0,
        image_input: 2,
        text_output: 7,
        reasoning_output: 3,
        read_cache: 11,
        write_cache: 12,
        write_cache_5m: 7,
        write_cache_1h: 3,
      })
    )
    const tokens = resolveDisplayTokens(billing)
    const rows = buildTokenTooltipRows(tokens)

    assert.deepEqual(
      rows.filter((row) => row.labelKey === 'usageLogs.fields.inputTokens'),
      [{ labelKey: 'usageLogs.fields.inputTokens', value: 0 }]
    )
    assert.ok(
      rows.some(
        (row) =>
          row.labelKey === 'usageLogs.fields.imageInput' && row.value === 2
      )
    )
    assert.ok(
      rows.some(
        (row) =>
          row.labelKey === 'usageLogs.fields.reasoningOutput' && row.value === 3
      )
    )
    assert.ok(
      rows.some(
        (row) =>
          row.labelKey === 'usageLogs.fields.cacheCreationUnallocated' &&
          row.value === 2
      )
    )
  })

  test('missing tooltips omit unavailable dimensions', () => {
    const tokens = resolveDisplayTokens(parseBillingDetails(null))
    const rows = buildTokenTooltipRows(tokens)

    assert.deepEqual(rows, [])
  })
})

describe('buildTokenBreakdownGroups', () => {
  test('separates official modalities, output audit splits and cache tiers', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 0,
        image_input: 2,
        text_output: 7,
        reasoning_output: 3,
        rejected_prediction: 1,
        read_cache: 11,
        write_cache: 12,
        write_cache_5m: 7,
        write_cache_1h: 3,
      })
    )
    const groups = buildTokenBreakdownGroups(resolveDisplayTokens(billing), {
      aggregatePromptTokens: 999,
      formatTokens: String,
    })

    const modality = groups.find(
      (group) => group.titleKey === 'usageLogs.fields.multimodalTokens'
    )
    const outputSplits = groups.find(
      (group) => group.titleKey === 'usageLogs.fields.outputSplitTokens'
    )
    assert.deepEqual(
      modality?.rows.map((row) => row.labelKey),
      ['usageLogs.fields.imageInput']
    )
    assert.deepEqual(outputSplits?.rows, [
      {
        labelKey: 'usageLogs.fields.reasoningOutput',
        value: '3',
      },
      {
        labelKey: 'usageLogs.fields.rejectedPrediction',
        value: '1',
      },
    ])
    assert.ok(
      groups
        .find((group) => group.titleKey === 'usageLogs.fields.cacheTokens')
        ?.rows.some(
          (row) =>
            row.labelKey === 'usageLogs.fields.cacheCreationUnallocated' &&
            row.value === '2'
        )
    )
  })

  test('missing token dimensions render placeholders without aggregate fallback', () => {
    const groups = buildTokenBreakdownGroups(
      resolveDisplayTokens(parseBillingDetails(null)),
      { aggregatePromptTokens: 120, formatTokens: String }
    )
    const standard = groups.find(
      (group) => group.titleKey === 'usageLogs.fields.standardTokens'
    )

    assert.deepEqual(standard?.rows.slice(0, 2), [
      { labelKey: 'usageLogs.fields.inputTokens', value: '-' },
      { labelKey: 'usageLogs.fields.outputTokens', value: '-' },
    ])
  })
})

describe('price snapshot helpers', () => {
  test('map official snapshot components with explicit zero quantities', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 12,
        reasoning_output: 3,
        read_cache: 4,
        write_cache: 5,
        write_cache_5m: 5,
      })
    )
    const tokens = resolveDisplayTokens(billing)

    assert.equal(
      getPriceSnapshotComponentQuantity('text_input', tokens, String),
      '12'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('reasoning_output', tokens, String),
      '3'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('write_cache', tokens, String),
      '5'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('request', tokens, String),
      '1'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('image_output', tokens, String),
      '0'
    )
    assert.equal(
      getPriceSnapshotComponentLabelKey('read_cache'),
      'systemSettings.fields.cacheRead'
    )
    assert.equal(
      getPriceSnapshotComponentLabelKey('custom_component'),
      'usageLogs.fields.billingItem'
    )
  })

  test('map contract snapshot component aliases without recalculation', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 12,
        text_output: 7,
        reasoning_output: 3,
        read_cache: 4,
        write_cache: 8,
        write_cache_5m: 5,
        write_cache_1h: 3,
      })
    )
    const tokens = resolveDisplayTokens(billing)

    assert.equal(
      getPriceSnapshotComponentQuantity('input', tokens, String),
      '12'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('output', tokens, String),
      '7'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('cache_read', tokens, String),
      '4'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('cache_write_5m', tokens, String),
      '5'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('cache_write_1h', tokens, String),
      '3'
    )
    assert.equal(
      getPriceSnapshotComponentLabelKey('cache_read'),
      'systemSettings.fields.cacheRead'
    )
    assert.equal(
      getPriceSnapshotComponentLabelKey('cache_write_5m'),
      'usageLogs.fields.cacheCreation5m'
    )
  })

  test('prefer saved settlement quantities over display token projection', () => {
    const billing = parseBillingDetails(
      completeBillingDetailsJson({
        text_input: 12,
        reasoning_output: 3,
        write_cache: 12,
        write_cache_5m: 7,
        write_cache_1h: 3,
      })
    )
    const tokens = resolveDisplayTokens(billing)

    assert.equal(
      getPriceSnapshotComponentQuantity('input', tokens, String, 988),
      '988'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('cache_write_5m', tokens, String, 9),
      '9'
    )
    assert.equal(
      getPriceSnapshotComponentQuantity('reasoning_output', tokens, String, -1),
      '—'
    )
  })

  test('display snapshot unit prices without currency conversion', () => {
    assert.equal(
      formatPriceSnapshotUnitPrice({
        unit_price: ' 4.2500 ',
        unit: 'per_1m_tokens',
        currency: 'EUR',
      }),
      '4.2500/M EUR'
    )
    assert.equal(
      formatPriceSnapshotUnitPrice({
        unit_price: '1.50',
        unit: 'per_request',
        currency: 'JPY',
      }),
      '1.50 JPY'
    )
    assert.equal(formatPriceSnapshotUnitPrice({ unit_price: ' ' }), null)
    assert.equal(
      formatPriceSnapshotUnitPrice({ unit_price: '4.25', currency: 'USD' }),
      '4.25 USD'
    )
    assert.equal(
      formatPriceSnapshotUnitPrice({
        unit_price: '4.25',
        unit: 'unsupported',
        currency: 'USD',
      }),
      '4.25 USD'
    )
  })
})
