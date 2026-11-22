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
  type ModelRatioMaps,
  applyBuiltinPricing,
  hasBuiltinPricing,
  modelRatioFields,
} from './builtin-pricing'

function emptyMaps(): ModelRatioMaps {
  return {
    ModelPrice: {},
    ModelRatio: {},
    CacheRatio: {},
    CreateCacheRatio: {},
    CompletionRatio: {},
    AudioRatio: {},
    AudioCompletionRatio: {},
    ContextPricing: {},
  }
}

describe('applyBuiltinPricing', () => {
  test('replaces every existing entry of the model and keeps other models', () => {
    const maps = emptyMaps()
    maps.ModelRatio['gpt-4o-mini'] = 0.075
    maps.ModelPrice['gpt-4o'] = 0.5
    maps.ModelRatio['gpt-4o'] = 9
    maps.CacheRatio['gpt-4o'] = 0.9
    maps.ContextPricing['other'] = { enabled: true, tiers: [] }
    const original = structuredClone(maps)

    const next = applyBuiltinPricing(maps, 'gpt-4o', {
      model_name: 'gpt-4o',
      available: true,
      model_ratio: 1.25,
      completion_ratio: 4,
      cache_ratio: 0.5,
    })

    assert.deepEqual(next.ModelPrice, {})
    assert.deepEqual(next.ModelRatio, { 'gpt-4o-mini': 0.075, 'gpt-4o': 1.25 })
    assert.deepEqual(next.CompletionRatio, { 'gpt-4o': 4 })
    assert.deepEqual(next.CacheRatio, { 'gpt-4o': 0.5 })
    assert.deepEqual(next.CreateCacheRatio, {})
    assert.deepEqual(next.AudioRatio, {})
    assert.deepEqual(next.AudioCompletionRatio, {})
    assert.deepEqual(next.ContextPricing, {
      other: { enabled: true, tiers: [] },
    })
    assert.deepEqual(maps, original)
  })

  test('writes per-request price, audio ratios and tiered context pricing', () => {
    const contextPricing = {
      enabled: true,
      tiers: [
        {
          name: 'base',
          min_tokens: 0,
          max_tokens: 200000,
          model_ratio: 1.5,
          completion_ratio: 5,
          cache_ratio: 0.1,
          create_cache_ratio: 1.25,
          audio_ratio: 1,
          audio_completion_ratio: 1,
        },
      ],
    }

    const next = applyBuiltinPricing(emptyMaps(), 'm', {
      model_name: 'm',
      available: true,
      model_price: 0.3,
      audio_ratio: 12.8,
      audio_completion_ratio: 2,
      context_pricing: contextPricing,
    })

    assert.deepEqual(next.ModelPrice, { m: 0.3 })
    assert.deepEqual(next.AudioRatio, { m: 12.8 })
    assert.deepEqual(next.AudioCompletionRatio, { m: 2 })
    assert.deepEqual(next.ContextPricing, { m: contextPricing })
    assert.deepEqual(next.ModelRatio, {})
  })

  test('covers all eight pricing maps', () => {
    assert.deepEqual(modelRatioFields, [
      'ModelPrice',
      'ModelRatio',
      'CacheRatio',
      'CreateCacheRatio',
      'CompletionRatio',
      'AudioRatio',
      'AudioCompletionRatio',
      'ContextPricing',
    ])
  })
})

describe('hasBuiltinPricing', () => {
  test('narrows only when the model has billable built-in entries', () => {
    assert.equal(
      hasBuiltinPricing({ model_name: 'm', available: false }),
      false
    )
    assert.equal(hasBuiltinPricing({ model_name: 'm', available: true }), true)
  })
})
