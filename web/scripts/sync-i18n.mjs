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
import fs from 'node:fs/promises'
import path from 'node:path'

// This script is executed from the web/ package root (see package.json script).
const LOCALES_DIR = path.resolve('src/i18n/locales')
const FALLBACK_COMPARE_LOCALE = 'en' // used for "still English" detection only

// i18next is configured with defaultNS "translation" and nsSeparator:false (see src/i18n/config.ts),
// so every translation key lives under a single flat namespace. Locales are physically split into
// per-feature section files (see src/i18n/locales/<locale>/<section>.json); this script merges them
// back into the single `translation` namespace for comparison/sync, then splits the result on write.
const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const BRAND_AND_LITERAL_KEYS = new Set([
  'AI Proxy',
  'AIGC2D',
  'Access Token',
  'Alipay',
  'Anthropic',
  'API Key',
  'API Key *',
  'API URL',
  'API2GPT',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'Baidu V2',
  'Base URL',
  'ByteDance',
  'CC Switch',
  'Chat Completions',
  'ChatGPT',
  'Claude',
  'Client ID',
  'Client Secret',
  'Cloudflare',
  'Cohere',
  'DeepSeek',
  'Discord',
  'Discord ID',
  'DoubaoVideo',
  'Embeddings',
  'FastGPT',
  'Function Calling',
  'Gemini',
  'Gemini Image 4K',
  'GitHub',
  'GitHub Client ID',
  'GitHub Client Secret',
  'GitHub ID',
  'Google',
  'Goroutines',
  'Jimeng',
  'JustSong',
  'LingYiWanWu',
  'LinuxDO',
  'LinuxDO Client ID',
  'LinuxDO Client Secret',
  'LinuxDO ID',
  'Logo URL',
  'Midjourney',
  'MidjourneyPlus',
  'Midjourney-Proxy',
  'MiniMax',
  'Mistral',
  'Mistral AI',
  'MokaAI',
  'Moonshot',
  'Moonshot AI',
  'NookMux',
  'NookMux <noreply@example.com>',
  'NookMux &lt;noreply@example.com&gt;',
  'OAuth Client Secret',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAIMax',
  'OpenRouter',
  'Pancake',
  'Passkey',
  'Perplexity',
  'Playground',
  'QuantumNous',
  'Quota:',
  'Replicate',
  'Rerank',
  'Responses',
  'Secret Key',
  'SentencePiece (Gemini)',
  'SiliconFlow',
  'Site Key',
  'Stability AI',
  'Stripe',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Telegram ID',
  'Tencent',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Uptime Kuma',
  'Uptime Kuma URL',
  'Vertex AI',
  'VolcEngine',
  'Waffo Pancake Dashboard',
  'Waffo Pancake MoR',
  'WeChat',
  'WeChat Pay',
  'Webhook URL',
  'Webhook URL:',
  'Well-Known URL',
  'Worker URL',
  'Xinference',
  'Xunfei',
  'Zhipu V4',
  '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"',
  'edit_this',
  'footer.columns.related.links.midjourney',
  'footer.columns.related.links.newApiKeyTool',
  'my-status',
  'new-api-key-tool',
  'price_xxx',
  'whsec_xxx',
])

function isPlainObject(v) {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function stableStringify(obj) {
  let text = JSON.stringify(obj, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return text + '\n'
}

function countLeafKeys(obj) {
  if (Array.isArray(obj)) return obj.length
  if (!isPlainObject(obj)) return 0
  let count = 0
  for (const k of Object.keys(obj)) {
    const v = obj[k]
    if (isPlainObject(v) || Array.isArray(v)) count += countLeafKeys(v)
    else count += 1
  }
  return count
}

function reorderLikeBase(base, target, fill, extras, missing, currentPath = []) {
  // If base is an object, we keep base's key order and recurse.
  if (isPlainObject(base)) {
    const out = {}
    const t = isPlainObject(target) ? target : {}
    const f = isPlainObject(fill) ? fill : {}

    for (const key of Object.keys(base)) {
      const nextPath = [...currentPath, key]
      if (Object.prototype.hasOwnProperty.call(t, key)) {
        out[key] = reorderLikeBase(base[key], t[key], f[key], extras, missing, nextPath)
      } else {
        missing.push(nextPath.join('.'))
        out[key] = reorderLikeBase(base[key], undefined, f[key], extras, missing, nextPath)
      }
    }

    for (const key of Object.keys(t)) {
      if (!Object.prototype.hasOwnProperty.call(base, key)) {
        const nextPath = [...currentPath, key].join('.')
        extras[nextPath] = t[key]
      }
    }

    return out
  }

  // For arrays: prefer target if it's also an array; otherwise use base.
  if (Array.isArray(base)) {
    if (Array.isArray(target)) return target
    if (Array.isArray(fill)) return fill
    return base
  }

  // For primitives: prefer target if defined, else base.
  return target === undefined ? (fill ?? base) : target
}

function isLikelyUntranslated({ locale, baseValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (value !== baseValue) return false

  // Skip short tokens / acronyms / ids
  const s = baseValue.trim()
  if (BRAND_AND_LITERAL_KEYS.has(s)) return false
  if (
    /^https?:\/\//.test(s) ||
    /^\/[\w/-]+/.test(s) ||
    /^[\w.-]+@[\w.-]+$/.test(s) ||
    /^smtp\./i.test(s) ||
    /^socks5:/i.test(s) ||
    /^org-/.test(s) ||
    /^gpt-/i.test(s) ||
    /^checkout\./.test(s) ||
    /^footer\./.test(s) ||
    /^[A-Z0-9_ *./:-]+$/.test(s) ||
    s.startsWith('{') ||
    s.startsWith('[') ||
    s.includes('&#10;') ||
    s.includes('\n')
  ) {
    return false
  }
  if (s.length < 6) return false
  if (!/[A-Za-z]{3,}/.test(s)) return false

  // For locales with non-latin scripts, equality with EN is a strong signal.
  if (locale === 'ja' || locale === 'zh') return true
  if (locale === 'ru') return true

  // For fr/vi: still useful but noisier; keep it conservative.
  if (locale === 'fr' || locale === 'vi') return /\b(the|and|or|to|with|please)\b/i.test(s)

  return false
}

// Locales are now physically split into per-feature section files under
// `<LOCALES_DIR>/<locale>/<section>.json`. Each section is a flat
// `Record<string, string>` (no `translation` wrapper). This module loads all
// sections for a locale, merges them into the legacy `{ translation: {...} }`
// shape so the rest of the sync logic stays unchanged, and splits the result
// back into section files on write.
//
// Membership of a key to a section is sticky: on write we reuse the section a
// key lived in before sync. Keys newly introduced by the base locale (and thus
// unknown to a non-base locale) are placed using the base locale's section map.
// Keys that are completely unknown go to `common.json`.

const SKIP_DIRS = new Set(['_extras', '_reports', 'node_modules'])
const FALLBACK_SECTION = 'common'

function assertFlatStringMap(sectionFile, obj) {
  if (!isPlainObject(obj)) {
    throw new Error(`Section file ${sectionFile} must be a flat JSON object of string -> string.`)
  }
  for (const [k, v] of Object.entries(obj)) {
    if (typeof v !== 'string') {
      throw new Error(
        `Section file ${sectionFile} has non-string value for key "${k}". Translation values must be strings.`,
      )
    }
  }
}

async function discoverLocaleDirs() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  return entries
    .filter((e) => e.isDirectory() && !SKIP_DIRS.has(e.name))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))
}

async function loadLocaleSections(locale) {
  const dir = path.join(LOCALES_DIR, locale)
  const entries = await fs.readdir(dir, { withFileTypes: true })
  const files = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))
  const sections = {} // sectionName -> { [key]: value }
  const sectionByKey = new Map() // key -> sectionName (first writer wins)
  for (const filename of files) {
    const sectionName = filename.replace(/\.json$/i, '')
    const full = path.join(dir, filename)
    const raw = await fs.readFile(full, 'utf8')
    const parsed = raw.trim().length === 0 ? {} : JSON.parse(raw)
    assertFlatStringMap(`${locale}/${filename}`, parsed)
    sections[sectionName] = {}
    for (const [k, v] of Object.entries(parsed)) {
      if (sectionByKey.has(k)) {
        // Duplicate across sections: keep first occurrence, surface the dup
        // via the extras mechanism so it is visible in the report rather than
        // silently dropped.
        continue
      }
      sectionByKey.set(k, sectionName)
      sections[sectionName][k] = v
    }
  }
  return { sections, sectionByKey }
}

function mergeSectionsToTranslation(sections) {
  const translation = {}
  for (const sectionName of Object.keys(sections)) {
    const s = sections[sectionName]
    for (const [k, v] of Object.entries(s)) translation[k] = v
  }
  return { translation }
}

// Split a reordered translation object back into the locale's section files.
// `localSectionByKey` is the locale's own pre-sync membership; for keys missing
// from it (e.g. newly added base keys), fall back to `baseSectionByKey`.
function splitTranslationToSections(trans, localSectionByKey, baseSectionByKey) {
  const out = {}
  for (const [k, v] of Object.entries(trans)) {
    const section =
      localSectionByKey.get(k) ?? baseSectionByKey.get(k) ?? FALLBACK_SECTION
    if (!out[section]) out[section] = {}
    out[section][k] = v
  }
  return out
}

async function main() {
  const localeDirs = await discoverLocaleDirs()
  if (localeDirs.length === 0) throw new Error('No locale directories found.')

  // Load each locale: section files -> merged { translation: {...} }, plus the
  // sticky key -> section membership map needed for the split write-back.
  const parsedByLocale = {}
  const sectionByKeyByLocale = {} // locale -> Map<key, sectionName>
  for (const locale of localeDirs) {
    const { sections, sectionByKey } = await loadLocaleSections(locale)
    parsedByLocale[locale] = mergeSectionsToTranslation(sections)
    sectionByKeyByLocale[locale] = sectionByKey
  }

  // Auto-pick base locale as the one with the most leaf keys under translation (most "rich").
  const baseLocale = Object.keys(parsedByLocale)
    .map((locale) => {
      const json = parsedByLocale[locale]
      const trans = json?.translation ?? {}
      return { locale, score: countLeafKeys(trans) }
    })
    .sort((a, b) => b.score - a.score || a.locale.localeCompare(b.locale))[0]?.locale

  if (!baseLocale) throw new Error('No locale directories found.')

  const baseDir = `${baseLocale}/`
  const baseJson = parsedByLocale[baseLocale]
  const baseSectionByKey = sectionByKeyByLocale[baseLocale]

  const compareJson = parsedByLocale[FALLBACK_COMPARE_LOCALE] ?? baseJson

  const report = {
    base: baseDir,
    locales: {},
  }

  const extrasDir = path.join(LOCALES_DIR, '_extras')
  const reportsDir = path.join(LOCALES_DIR, '_reports')
  await fs.mkdir(extrasDir, { recursive: true })
  await fs.mkdir(reportsDir, { recursive: true })

  for (const locale of localeDirs) {
    const localeDir = path.join(LOCALES_DIR, locale)
    const json = parsedByLocale[locale]
    const localSectionByKey = sectionByKeyByLocale[locale]

    const extras = {}
    const missing = []
    const fixed = reorderLikeBase(baseJson, json, compareJson, extras, missing)

    // Untranslated scan (translation namespace only)
    const untranslated = {}
    const compareTrans = compareJson?.translation ?? {}
    const trans = fixed?.translation ?? {}
    if (
      isPlainObject(compareTrans) &&
      isPlainObject(trans) &&
      locale !== FALLBACK_COMPARE_LOCALE &&
      locale !== baseLocale
    ) {
      for (const k of Object.keys(compareTrans)) {
        const baseValue = compareTrans[k]
        const value = trans[k]
        if (isLikelyUntranslated({ locale, baseValue, value })) {
          untranslated[k] = value
        }
      }
    }

    report.locales[locale] = {
      dir: `${locale}/`,
      missingCount: missing.length,
      extrasCount: Object.keys(extras).length,
      untranslatedCount: Object.keys(untranslated).length,
    }

    if (Object.keys(extras).length > 0) {
      await fs.writeFile(path.join(extrasDir, `${locale}.extras.json`), stableStringify(extras), 'utf8')
    } else {
      await fs.rm(path.join(extrasDir, `${locale}.extras.json`), { force: true })
    }
    if (Object.keys(untranslated).length > 0) {
      await fs.writeFile(
        path.join(reportsDir, `${locale}.untranslated.json`),
        stableStringify(untranslated),
        'utf8',
      )
    } else {
      await fs.rm(path.join(reportsDir, `${locale}.untranslated.json`), { force: true })
    }

    // Split the reordered translation back into the locale's section files,
    // preserving pre-existing section membership and reusing the base locale's
    // membership for keys newly introduced in this sync.
    const sectionsOut = splitTranslationToSections(
      trans,
      localSectionByKey,
      baseSectionByKey,
    )

    // Remove any section files that no longer have keys (e.g. after a rename or
    // full migration) to avoid stale leftovers.
    const existingEntries = await fs.readdir(localeDir, { withFileTypes: true })
    const existingSectionFiles = existingEntries
      .filter((e) => e.isFile() && e.name.endsWith('.json'))
      .map((e) => e.name)
    for (const filename of existingSectionFiles) {
      const sectionName = filename.replace(/\.json$/i, '')
      if (!Object.prototype.hasOwnProperty.call(sectionsOut, sectionName)) {
        await fs.rm(path.join(localeDir, filename), { force: true })
      }
    }
    for (const [sectionName, body] of Object.entries(sectionsOut)) {
      await fs.writeFile(path.join(localeDir, `${sectionName}.json`), stableStringify(body), 'utf8')
    }
  }

  await fs.writeFile(path.join(reportsDir, '_sync-report.json'), stableStringify(report), 'utf8')

  console.log(`i18n sync done. Report: ${path.join(reportsDir, '_sync-report.json')}`)
}

main().catch((err) => {
   
  console.error(err)
  process.exitCode = 1
})


