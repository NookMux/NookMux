import assert from 'node:assert/strict'
import { afterEach, describe, test } from 'node:test'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyConfig,
} from '@/stores/system-config-store'
import {
  formatCompactCurrencyFromUSD,
  formatCompactQuotaWithCurrency,
  formatCurrencyFromUSD,
} from './currency'

const initialConfig = useSystemConfigStore.getState().config

afterEach(() => {
  useSystemConfigStore.setState({ config: initialConfig })
})

function setDisplay(currency: Partial<CurrencyConfig>) {
  useSystemConfigStore.setState({
    config: {
      ...initialConfig,
      currency: { ...DEFAULT_CURRENCY_CONFIG, ...currency },
    },
  })
}

describe('formatCompactCurrencyFromUSD', () => {
  test('abbreviates large values with truncated K/M/B suffixes', () => {
    setDisplay({ quotaDisplayType: 'USD' })
    assert.equal(formatCompactCurrencyFromUSD(33678.86), '$33.67K')
    assert.equal(formatCompactCurrencyFromUSD(1146.13), '$1.14K')
    assert.equal(formatCompactCurrencyFromUSD(12345678), '$12.34M')
    assert.equal(formatCompactCurrencyFromUSD(1234567890), '$1.23B')
  })

  test('never rounds up across a tier boundary', () => {
    setDisplay({ quotaDisplayType: 'USD' })
    assert.equal(formatCompactCurrencyFromUSD(99999983.93), '$99.99M')
    assert.equal(formatCompactCurrencyFromUSD(999999999999), '$999.99B')
  })

  test('trims trailing zeros from the abbreviated magnitude', () => {
    setDisplay({ quotaDisplayType: 'USD' })
    assert.equal(formatCompactCurrencyFromUSD(33000), '$33K')
    assert.equal(formatCompactCurrencyFromUSD(1200000), '$1.2M')
  })

  test('falls back to the exact formatter below 1000', () => {
    setDisplay({ quotaDisplayType: 'USD' })
    assert.equal(
      formatCompactCurrencyFromUSD(202.37),
      formatCurrencyFromUSD(202.37)
    )
    assert.equal(
      formatCompactCurrencyFromUSD(0.0153),
      formatCurrencyFromUSD(0.0153)
    )
    assert.equal(formatCompactCurrencyFromUSD(0), formatCurrencyFromUSD(0))
  })

  test('places the negative sign before the currency symbol', () => {
    setDisplay({ quotaDisplayType: 'USD' })
    assert.equal(formatCompactCurrencyFromUSD(-33678.86), '-$33.67K')
  })

  test('converts through the exchange rate in CNY mode', () => {
    setDisplay({ quotaDisplayType: 'CNY', usdExchangeRate: 7 })
    assert.equal(formatCompactCurrencyFromUSD(5000), '¥35K')
    assert.equal(formatCompactCurrencyFromUSD(-5000), '-¥35K')
  })

  test('abbreviates token counts in TOKENS mode', () => {
    setDisplay({
      quotaDisplayType: 'TOKENS',
      quotaPerUnit: 500000,
    })
    assert.equal(formatCompactCurrencyFromUSD(200), '100M')
    assert.equal(formatCompactCurrencyFromUSD(0.004), '2K')
  })
})

describe('formatCompactQuotaWithCurrency', () => {
  test('converts raw quota units then abbreviates', () => {
    setDisplay({ quotaDisplayType: 'USD', quotaPerUnit: 500000 })
    assert.equal(formatCompactQuotaWithCurrency(16839430000), '$33.67K')
  })

  test('returns "-" for missing values like the exact formatter', () => {
    assert.equal(formatCompactQuotaWithCurrency(null), '-')
    assert.equal(formatCompactCurrencyFromUSD(undefined), '-')
  })
})
