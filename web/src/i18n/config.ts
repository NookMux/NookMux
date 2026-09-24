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
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
// Locale strings are physically split per-feature under locales/<lng>/<section>.json
// to keep individual files maintainable. They are merged back here into the exact
// i18next resources shape that consumers depend on (single `translation` namespace,
// flat keys, keySeparator: false, nsSeparator: false). No call site changes.
//
// To add a new section: drop the file under locales/{en,zh}/, import it below, and
// spread it into the matching `translation` object.
import enAbout from './locales/en/about.json'
import enAuditLogs from './locales/en/audit-logs.json'
import enAuth from './locales/en/auth.json'
import enChannels from './locales/en/channels.json'
import enCommon from './locales/en/common.json'
import enDashboard from './locales/en/dashboard.json'
import enDynamicRatio from './locales/en/dynamic-ratio.json'
import enHome from './locales/en/home.json'
import enKeyQuery from './locales/en/key-query.json'
import enKeys from './locales/en/keys.json'
import enLayout from './locales/en/layout.json'
import enModels from './locales/en/models.json'
import enMultimodalFiles from './locales/en/multimodal-files.json'
import enMultimodal from './locales/en/multimodal.json'
import enOrderQuery from './locales/en/order-query.json'
import enPlayground from './locales/en/playground.json'
import enPricing from './locales/en/pricing.json'
import enProfile from './locales/en/profile.json'
import enRankings from './locales/en/rankings.json'
import enRedemptionCodes from './locales/en/redemption-codes.json'
import enSetup from './locales/en/setup.json'
import enSubscriptions from './locales/en/subscriptions.json'
import enSystemSettings from './locales/en/system-settings.json'
import enTickets from './locales/en/tickets.json'
import enUsageLogs from './locales/en/usage-logs.json'
import enUsers from './locales/en/users.json'
import enVoiceManagement from './locales/en/voice-management.json'
import enWallet from './locales/en/wallet.json'
import zhAbout from './locales/zh/about.json'
import zhAuditLogs from './locales/zh/audit-logs.json'
import zhAuth from './locales/zh/auth.json'
import zhChannels from './locales/zh/channels.json'
import zhCommon from './locales/zh/common.json'
import zhDashboard from './locales/zh/dashboard.json'
import zhDynamicRatio from './locales/zh/dynamic-ratio.json'
import zhHome from './locales/zh/home.json'
import zhKeyQuery from './locales/zh/key-query.json'
import zhKeys from './locales/zh/keys.json'
import zhLayout from './locales/zh/layout.json'
import zhModels from './locales/zh/models.json'
import zhMultimodalFiles from './locales/zh/multimodal-files.json'
import zhMultimodal from './locales/zh/multimodal.json'
import zhOrderQuery from './locales/zh/order-query.json'
import zhPlayground from './locales/zh/playground.json'
import zhPricing from './locales/zh/pricing.json'
import zhProfile from './locales/zh/profile.json'
import zhRankings from './locales/zh/rankings.json'
import zhRedemptionCodes from './locales/zh/redemption-codes.json'
import zhSetup from './locales/zh/setup.json'
import zhSubscriptions from './locales/zh/subscriptions.json'
import zhSystemSettings from './locales/zh/system-settings.json'
import zhTickets from './locales/zh/tickets.json'
import zhUsageLogs from './locales/zh/usage-logs.json'
import zhUsers from './locales/zh/users.json'
import zhVoiceManagement from './locales/zh/voice-management.json'
import zhWallet from './locales/zh/wallet.json'

const en = {
  translation: {
    ...enAbout,
    ...enAuditLogs,
    ...enAuth,
    ...enChannels,
    ...enCommon,
    ...enDashboard,
    ...enDynamicRatio,
    ...enHome,
    ...enKeyQuery,
    ...enKeys,
    ...enLayout,
    ...enModels,
    ...enMultimodalFiles,
    ...enMultimodal,
    ...enOrderQuery,
    ...enPlayground,
    ...enPricing,
    ...enProfile,
    ...enRankings,
    ...enRedemptionCodes,
    ...enSetup,
    ...enSubscriptions,
    ...enSystemSettings,
    ...enTickets,
    ...enUsageLogs,
    ...enUsers,
    ...enVoiceManagement,
    ...enWallet,
  },
} as const

const zh = {
  translation: {
    ...zhAbout,
    ...zhAuditLogs,
    ...zhAuth,
    ...zhChannels,
    ...zhCommon,
    ...zhDashboard,
    ...zhDynamicRatio,
    ...zhHome,
    ...zhKeyQuery,
    ...zhKeys,
    ...zhLayout,
    ...zhModels,
    ...zhMultimodalFiles,
    ...zhMultimodal,
    ...zhOrderQuery,
    ...zhPlayground,
    ...zhPricing,
    ...zhProfile,
    ...zhRankings,
    ...zhRedemptionCodes,
    ...zhSetup,
    ...zhSubscriptions,
    ...zhSystemSettings,
    ...zhTickets,
    ...zhUsageLogs,
    ...zhUsers,
    ...zhVoiceManagement,
    ...zhWallet,
  },
} as const

export const resources = {
  en,
  zh,
} as const

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh'],
    load: 'languageOnly', // Convert zh-CN -> zh
    keySeparator: false, // Allow flat translation keys containing dots
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
    },
  })

// Keep <html lang="..."> in sync with the resolved language so browsers
// don't offer to translate a page whose declared lang differs from the
// actual content language (e.g. lang="en" with Chinese text → translate popup).
function syncDocumentLang(lng: string) {
  document.documentElement.lang = lng
}
i18n.on('languageChanged', syncDocumentLang)
syncDocumentLang(i18n.language)

export default i18n
