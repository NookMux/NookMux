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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DashboardLimitsSection } from '../dashboard/components/dashboard-limits-section'
import { DashboardMetricsSection } from '../dashboard/components/dashboard-metrics-section'
import { DashboardPanelsSection } from '../dashboard/components/dashboard-panels-section'
import { DashboardRefreshSection } from '../dashboard/components/dashboard-refresh-section'

type DashboardTab = 'metrics' | 'panels' | 'refresh' | 'limits'

const TABS: { id: DashboardTab; titleKey: string }[] = [
  { id: 'metrics', titleKey: 'systemSettings.fields.dataMetrics' },
  { id: 'panels', titleKey: 'systemSettings.titles.dashboardPanels' },
  { id: 'refresh', titleKey: 'systemSettings.actions.refreshIntervals' },
  { id: 'limits', titleKey: 'systemSettings.fields.dataLimits' },
]

// 在「控制台内容」分组下作为单个分区渲染，页面标题与页头操作栏由系统设置的页面框架提供。
// 四个页签用本地状态切换，不依赖嵌套路由的 useParams，避免 TanStack Router invariant 失败。
// 「数据指标」「数据看板面板」的开关切换后立即生效；「刷新间隔」「数据限制」是表单，
// 两者的「重置」「保存」通过 SettingsPageFormActions 挂到页面框架的页头操作栏。
export function DashboardConfigSection() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<DashboardTab>('metrics')

  return (
    <div className='flex w-full min-w-0 flex-col gap-4'>
      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as DashboardTab)}
      >
        <TabsList variant='line'>
          {TABS.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id}>
              {t(tab.titleKey)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      {activeTab === 'metrics' && <DashboardMetricsSection />}
      {activeTab === 'panels' && <DashboardPanelsSection />}
      {activeTab === 'refresh' && <DashboardRefreshSection />}
      {activeTab === 'limits' && <DashboardLimitsSection />}
    </div>
  )
}
