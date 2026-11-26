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
import { useState, useMemo, useEffect, useCallback, memo } from 'react'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SettingsPanel } from '../components/settings-panel'
import { safeJsonParse } from '../utils/json-parser'

type GroupRatioVisualEditorProps = {
  groupRatio: string
  topupGroupRatio: string
  userUsableGroups: string
  groupGroupRatio: string
  autoGroups: string
  onChange: (field: string, value: string) => void
}

type SimpleGroup = {
  name: string
  value: string
}

type GroupPricingRow = {
  _id: string
  name: string
  ratio: number
  selectable: boolean
  description: string
}

type GroupOverride = {
  targetGroup: string
  ratio: number
}

let groupPricingIdCounter = 0
function createGroupPricingId() {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string
): GroupPricingRow[] {
  const ratioMap = safeJsonParse<Record<string, number>>(groupRatio, {
    fallback: {},
    context: 'group ratios',
  })
  const usableMap = safeJsonParse<Record<string, string>>(userUsableGroups, {
    fallback: {},
    context: 'user usable groups',
  })
  const names = new Set([...Object.keys(ratioMap), ...Object.keys(usableMap)])

  return Array.from(names).map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: normalizeRatio(ratioMap[name]),
    selectable: Object.prototype.hasOwnProperty.call(usableMap, name),
    description: String(usableMap[name] ?? ''),
  }))
}

function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    if (row.selectable) {
      userUsableGroups[name] = row.description
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  }
}

function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: safeJsonParse(serialized.GroupRatio, {
      fallback: {},
      silent: true,
    }),
    userUsableGroups: safeJsonParse(serialized.UserUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string
): string {
  return JSON.stringify({
    groupRatio: safeJsonParse(groupRatio, { fallback: {}, silent: true }),
    userUsableGroups: safeJsonParse(userUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

export const GroupRatioVisualEditor = memo(function GroupRatioVisualEditor({
  groupRatio,
  topupGroupRatio,
  userUsableGroups,
  groupGroupRatio,
  autoGroups,
  onChange,
}: GroupRatioVisualEditorProps) {
  const { t } = useTranslation()
  const [simpleDialogOpen, setSimpleDialogOpen] = useState(false)
  const [simpleDialogType, setSimpleDialogType] = useState<
    'groupRatio' | 'topupGroupRatio' | null
  >(null)
  const [simpleEditData, setSimpleEditData] = useState<SimpleGroup | null>(null)

  const [autoGroupDialogOpen, setAutoGroupDialogOpen] = useState(false)
  const [autoGroupInput, setAutoGroupInput] = useState('')

  const [groupOverrideDialogOpen, setGroupOverrideDialogOpen] = useState(false)
  const [groupOverrideUserGroup, setGroupOverrideUserGroup] = useState<
    string | null
  >(null)
  const [groupOverrideEditData, setGroupOverrideEditData] =
    useState<GroupOverride | null>(null)

  const [userGroupDialogOpen, setUserGroupDialogOpen] = useState(false)
  const [userGroupInput, setUserGroupInput] = useState('')

  // Parse topup group ratios
  const topupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, number>>(topupGroupRatio, {
      fallback: {},
      context: 'topup group ratios',
    })
    return Object.entries(map).map(([name, value]) => ({
      name,
      value: String(value),
    }))
  }, [topupGroupRatio])

  // Parse auto groups
  const autoGroupsList = useMemo(() => {
    return safeJsonParse<string[]>(autoGroups, {
      fallback: [],
      context: 'auto groups',
    })
  }, [autoGroups])

  // Parse group-group ratios
  const groupGroupRatioList = useMemo(() => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        context: 'group-group ratios',
      }
    )
    return Object.entries(map).map(([userGroup, overrides]) => ({
      userGroup,
      overrides: Object.entries(overrides).map(([targetGroup, ratio]) => ({
        targetGroup,
        ratio,
      })),
    }))
  }, [groupGroupRatio])

  // Simple group handlers (for groupRatio and topupGroupRatio)
  const handleSimpleAdd = (type: 'groupRatio' | 'topupGroupRatio') => {
    setSimpleDialogType(type)
    setSimpleEditData(null)
    setSimpleDialogOpen(true)
  }

  const handleSimpleEdit = (
    type: 'groupRatio' | 'topupGroupRatio',
    group: SimpleGroup
  ) => {
    setSimpleDialogType(type)
    setSimpleEditData(group)
    setSimpleDialogOpen(true)
  }

  const handleSimpleSave = (name: string, value: string) => {
    if (!simpleDialogType) return

    const fieldName =
      simpleDialogType === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })

    if (simpleEditData && simpleEditData.name !== name) {
      delete map[simpleEditData.name]
    }

    map[name] = parseFloat(value)

    const field =
      simpleDialogType === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
    setSimpleDialogOpen(false)
  }

  const handleSimpleDelete = (
    type: 'groupRatio' | 'topupGroupRatio',
    name: string
  ) => {
    const fieldName = type === 'groupRatio' ? groupRatio : topupGroupRatio
    const map = safeJsonParse<Record<string, number>>(fieldName, {
      fallback: {},
      silent: true,
    })
    delete map[name]

    const field = type === 'groupRatio' ? 'GroupRatio' : 'TopupGroupRatio'
    onChange(field, JSON.stringify(map, null, 2))
  }

  // Auto groups handlers
  const handleAutoGroupAdd = () => {
    setAutoGroupInput('')
    setAutoGroupDialogOpen(true)
  }

  const handleAutoGroupSave = () => {
    if (!autoGroupInput.trim()) return

    const list = [...autoGroupsList, autoGroupInput.trim()]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
    setAutoGroupDialogOpen(false)
  }

  const handleAutoGroupDelete = (index: number) => {
    const list = autoGroupsList.filter((_, i) => i !== index)
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  const handleAutoGroupMove = (index: number, direction: 'up' | 'down') => {
    const list = [...autoGroupsList]
    const newIndex = direction === 'up' ? index - 1 : index + 1

    if (newIndex < 0 || newIndex >= list.length) return
    ;[list[index], list[newIndex]] = [list[newIndex], list[index]]
    onChange('AutoGroups', JSON.stringify(list, null, 2))
  }

  // Group-group ratio handlers
  const handleUserGroupAdd = () => {
    setUserGroupInput('')
    setUserGroupDialogOpen(true)
  }

  const handleUserGroupSave = () => {
    if (!userGroupInput.trim()) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[userGroupInput.trim()]) {
      map[userGroupInput.trim()] = {}
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setUserGroupDialogOpen(false)
  }

  const handleUserGroupDelete = (userGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )
    delete map[userGroup]
    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  const handleOverrideAdd = (userGroup: string) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(null)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideEdit = (userGroup: string, override: GroupOverride) => {
    setGroupOverrideUserGroup(userGroup)
    setGroupOverrideEditData(override)
    setGroupOverrideDialogOpen(true)
  }

  const handleOverrideSave = (
    targetGroup: string,
    ratio: number,
    oldTargetGroup?: string
  ) => {
    if (!groupOverrideUserGroup) return

    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (!map[groupOverrideUserGroup]) {
      map[groupOverrideUserGroup] = {}
    }

    if (oldTargetGroup && oldTargetGroup !== targetGroup) {
      delete map[groupOverrideUserGroup][oldTargetGroup]
    }

    map[groupOverrideUserGroup][targetGroup] = ratio

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    setGroupOverrideDialogOpen(false)
  }

  const handleOverrideDelete = (userGroup: string, targetGroup: string) => {
    const map = safeJsonParse<Record<string, Record<string, number>>>(
      groupGroupRatio,
      {
        fallback: {},
        silent: true,
      }
    )

    if (map[userGroup]) {
      delete map[userGroup][targetGroup]
      if (Object.keys(map[userGroup]).length === 0) {
        delete map[userGroup]
      }
    }

    onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
  }

  return (
    <div className='flex min-w-0 flex-col gap-8'>
      <GroupPricingTable
        groupRatio={groupRatio}
        userUsableGroups={userUsableGroups}
        onChange={onChange}
      />

      {/* Topup Group Ratios */}
      <SettingsPanel
        title={t('systemSettings.fields.topUpGroupRatios')}
        description={t(
          'systemSettings.tips.multipliersForRechargePricingBasedOnUserGroups'
        )}
        actions={
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => handleSimpleAdd('topupGroupRatio')}
          >
            <Plus data-icon='inline-start' />
            {t('systemSettings.actions.addGroup')}
          </Button>
        }
        flush
      >
        {topupRatioList.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            {t('systemSettings.tips.noGroupsYetAddAGroupToGetStarted')}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='pl-4'>
                  {t('models.fields.groupName')}
                </TableHead>
                <TableHead>{t('systemSettings.fields.multiplier')}</TableHead>
                <TableHead className='pr-4 text-right'>
                  {t('channels.fields.actions')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {topupRatioList.map((group) => (
                <TableRow key={group.name}>
                  <TableCell className='pl-4 font-medium'>
                    {group.name}
                  </TableCell>
                  <TableCell className='tabular-nums'>{group.value}</TableCell>
                  <TableCell className='pr-4 text-right'>
                    <div className='flex justify-end gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        aria-label={t('channels.actions.edit')}
                        onClick={() =>
                          handleSimpleEdit('topupGroupRatio', group)
                        }
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        aria-label={t('common.actions.delete')}
                        onClick={() =>
                          handleSimpleDelete('topupGroupRatio', group.name)
                        }
                      >
                        <Trash2 className='text-destructive h-4 w-4' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </SettingsPanel>

      {/* Inter-group ratio overrides */}
      <SettingsPanel
        title={t('systemSettings.fields.interGroupRatioOverrides')}
        description={t(
          'common.tips.customMultipliersWhenSpecificUserGroupsUseSpecificToken'
        )}
        actions={
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleUserGroupAdd}
          >
            <Plus data-icon='inline-start' />
            {t('systemSettings.actions.addUserGroup')}
          </Button>
        }
      >
        {groupGroupRatioList.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            {t('systemSettings.tips.noUserGroupsYetAddAUserGroupToGetStarted')}
          </p>
        ) : (
          <div className='[&>*+*]:border-t'>
            {groupGroupRatioList.map((userGroupData) => (
              <Collapsible key={userGroupData.userGroup}>
                <div className='flex items-center justify-between py-2'>
                  <div className='flex min-w-0 items-center gap-2'>
                    <CollapsibleTrigger
                      render={
                        <Button
                          variant='ghost'
                          size='sm'
                          className='h-6 w-6 p-0'
                        />
                      }
                    >
                      <ChevronDown className='h-4 w-4' />
                    </CollapsibleTrigger>
                    <span className='text-sm leading-snug font-medium'>
                      {userGroupData.userGroup}
                    </span>
                    <span className='text-muted-foreground text-sm'>
                      {t('systemSettings.fields.countOverride', {
                        count: userGroupData.overrides.length,
                      })}
                    </span>
                  </div>
                  <div className='flex shrink-0 gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='h-7 w-7'
                      aria-label={t('systemSettings.actions.addRatioOverride')}
                      onClick={() => handleOverrideAdd(userGroupData.userGroup)}
                    >
                      <Plus className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='h-7 w-7'
                      aria-label={t('common.actions.delete')}
                      onClick={() =>
                        handleUserGroupDelete(userGroupData.userGroup)
                      }
                    >
                      <Trash2 className='text-destructive h-4 w-4' />
                    </Button>
                  </div>
                </div>
                <CollapsibleContent>
                  {userGroupData.overrides.length > 0 && (
                    <div className='ml-3 border-l pl-5 [&>*+*]:border-t'>
                      {userGroupData.overrides.map((override) => (
                        <div
                          key={override.targetGroup}
                          className='flex items-center gap-3 py-2'
                        >
                          <span className='min-w-0 flex-1 truncate text-sm font-medium'>
                            {override.targetGroup}
                          </span>
                          <span className='text-sm tabular-nums'>
                            {override.ratio}
                          </span>
                          <div className='flex shrink-0 gap-1'>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              className='h-7 w-7'
                              aria-label={t('channels.actions.edit')}
                              onClick={() =>
                                handleOverrideEdit(
                                  userGroupData.userGroup,
                                  override
                                )
                              }
                            >
                              <Pencil className='h-4 w-4' />
                            </Button>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              className='h-7 w-7'
                              aria-label={t('common.actions.delete')}
                              onClick={() =>
                                handleOverrideDelete(
                                  userGroupData.userGroup,
                                  override.targetGroup
                                )
                              }
                            >
                              <Trash2 className='text-destructive h-4 w-4' />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CollapsibleContent>
              </Collapsible>
            ))}
          </div>
        )}
      </SettingsPanel>

      {/* Auto Groups */}
      <SettingsPanel
        title={t('systemSettings.fields.autoAssignmentOrder')}
        description={t(
          'systemSettings.tips.priorityOrderForAutomaticGroupAssignmentNewTokensRotate'
        )}
        actions={
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleAutoGroupAdd}
          >
            <Plus data-icon='inline-start' />
            {t('systemSettings.actions.addGroup')}
          </Button>
        }
      >
        {autoGroupsList.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            {t('systemSettings.tips.noAutoGroupsYetAddAGroupToGetStarted')}
          </p>
        ) : (
          <div className='[&>*+*]:border-t'>
            {autoGroupsList.map((group, index) => (
              <div key={index} className='flex items-center gap-3 py-1.5'>
                <span className='text-muted-foreground w-5 text-right text-xs tabular-nums'>
                  {index + 1}
                </span>
                <span className='min-w-0 flex-1 truncate text-sm font-medium'>
                  {group}
                </span>
                <div className='flex gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-7 w-7'
                    disabled={index === 0}
                    aria-label={t('channels.actions.moveUp')}
                    onClick={() => handleAutoGroupMove(index, 'up')}
                  >
                    <ArrowUp className='h-4 w-4' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-7 w-7'
                    disabled={index === autoGroupsList.length - 1}
                    aria-label={t('channels.actions.moveDown')}
                    onClick={() => handleAutoGroupMove(index, 'down')}
                  >
                    <ArrowDown className='h-4 w-4' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-7 w-7'
                    aria-label={t('common.actions.delete')}
                    onClick={() => handleAutoGroupDelete(index)}
                  >
                    <Trash2 className='text-destructive h-4 w-4' />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </SettingsPanel>

      {/* Simple Group Dialog */}
      <SimpleGroupDialog
        open={simpleDialogOpen}
        onOpenChange={setSimpleDialogOpen}
        onSave={handleSimpleSave}
        editData={simpleEditData}
        type={simpleDialogType}
      />

      {/* Auto Group Dialog */}
      <Dialog open={autoGroupDialogOpen} onOpenChange={setAutoGroupDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t('systemSettings.actions.addAutoGroup')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.actions.addAGroupIdentifierToTheAutoAssignmentList'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label>{t('systemSettings.fields.groupIdentifier')}</Label>
              <Input
                value={autoGroupInput}
                onChange={(e) => setAutoGroupInput(e.target.value)}
                placeholder={t('common.fields.default')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setAutoGroupDialogOpen(false)}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button onClick={handleAutoGroupSave}>
              {t('channels.actions.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* User Group Dialog */}
      <Dialog open={userGroupDialogOpen} onOpenChange={setUserGroupDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t('systemSettings.actions.addUserGroup')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'systemSettings.actions.createANewUserGroupToConfigureRatioOverrides'
              )}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label>{t('systemSettings.fields.userGroupName')}</Label>
              <Input
                value={userGroupInput}
                onChange={(e) => setUserGroupInput(e.target.value)}
                placeholder={t('systemSettings.fields.vip')}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setUserGroupDialogOpen(false)}
            >
              {t('common.actions.cancel')}
            </Button>
            <Button onClick={handleUserGroupSave}>
              {t('channels.actions.add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Group Override Dialog */}
      <GroupOverrideDialog
        open={groupOverrideDialogOpen}
        onOpenChange={setGroupOverrideDialogOpen}
        onSave={handleOverrideSave}
        editData={groupOverrideEditData}
        userGroup={groupOverrideUserGroup}
      />
    </div>
  )
})

type GroupPricingTableProps = {
  groupRatio: string
  userUsableGroups: string
  onChange: (field: string, value: string) => void
}

function GroupPricingTable({
  groupRatio,
  userUsableGroups,
  onChange,
}: GroupPricingTableProps) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<GroupPricingRow[]>(() =>
    buildGroupPricingRows(groupRatio, userUsableGroups)
  )

  useEffect(() => {
    const incomingSignature = sourceGroupPricingSignature(
      groupRatio,
      userUsableGroups
    )
    setRows((currentRows) => {
      if (groupPricingSignature(currentRows) === incomingSignature) {
        return currentRows
      }
      return buildGroupPricingRows(groupRatio, userUsableGroups)
    })
  }, [groupRatio, userUsableGroups])

  const emitRows = useCallback(
    (nextRows: GroupPricingRow[]) => {
      setRows(nextRows)
      const serialized = serializeGroupPricingRows(nextRows)
      onChange('GroupRatio', serialized.GroupRatio)
      onChange('UserUsableGroups', serialized.UserUsableGroups)
    },
    [onChange]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GroupPricingRow, '_id'>,
      value: string | number | boolean
    ) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createGroupPricingId(),
        name,
        ratio: 1,
        selectable: true,
        description: '',
      },
    ])
  }, [emitRows, rows])

  const removeRow = useCallback(
    (id: string) => {
      emitRows(rows.filter((row) => row._id !== id))
    },
    [emitRows, rows]
  )

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return Array.from(counts.entries())
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  return (
    <SettingsPanel
      title={t('systemSettings.fields.pricingGroups')}
      description={t(
        'systemSettings.actions.editBillingRatiosAndUserSelectableGroupsInOne'
      )}
      actions={
        <Button type='button' variant='outline' size='sm' onClick={addRow}>
          <Plus data-icon='inline-start' />
          {t('systemSettings.actions.addGroup')}
        </Button>
      }
      flush
      footer={
        duplicateNames.length > 0 ? (
          <p className='text-destructive w-full text-sm'>
            {t('systemSettings.errors.duplicateGroupNamesNames', {
              names: duplicateNames.join(', '),
            })}
          </p>
        ) : undefined
      }
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='min-w-28 pl-4'>
              {t('models.fields.groupName')}
            </TableHead>
            <TableHead className='min-w-28'>
              {t('dynamicRatio.fields.ratio794f65')}
            </TableHead>
            {/* 窄容器下表头按词换行，不在词内断开。 */}
            <TableHead className='w-28 text-center [word-break:keep-all] whitespace-normal'>
              {t('systemSettings.fields.userSelectable')}
            </TableHead>
            <TableHead className='min-w-32'>
              {t('auditLogs.tips.description')}
            </TableHead>
            <TableHead className='w-16 pr-4 text-right'>
              {t('channels.fields.actions')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={5}
                className='text-muted-foreground h-20 text-center text-sm'
              >
                {t('systemSettings.tips.noGroupsYetAddAGroupToGetStarted')}
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow key={row._id}>
                <TableCell className='pl-4'>
                  <Input
                    value={row.name}
                    onChange={(event) =>
                      updateRow(row._id, 'name', event.target.value)
                    }
                    aria-invalid={duplicateNames.includes(row.name.trim())}
                  />
                </TableCell>
                <TableCell>
                  <Input
                    type='number'
                    min={0}
                    step={0.1}
                    value={String(row.ratio)}
                    onChange={(event) =>
                      updateRow(
                        row._id,
                        'ratio',
                        normalizeRatio(event.target.value)
                      )
                    }
                  />
                </TableCell>
                <TableCell>
                  <div className='flex justify-center'>
                    <Checkbox
                      checked={row.selectable}
                      onCheckedChange={(checked) =>
                        updateRow(row._id, 'selectable', checked === true)
                      }
                      aria-label={t('systemSettings.fields.userSelectable')}
                    />
                  </div>
                </TableCell>
                <TableCell>
                  {row.selectable ? (
                    <Input
                      value={row.description}
                      placeholder={t('systemSettings.tips.groupDescription')}
                      onChange={(event) =>
                        updateRow(row._id, 'description', event.target.value)
                      }
                    />
                  ) : (
                    <span className='text-muted-foreground px-3 text-sm'>
                      -
                    </span>
                  )}
                </TableCell>
                <TableCell className='pr-4 text-right'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={() => removeRow(row._id)}
                    aria-label={t('common.actions.delete')}
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </SettingsPanel>
  )
}

// Simple Group Dialog Component
type SimpleGroupDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (name: string, value: string) => void
  editData: SimpleGroup | null
  type: 'groupRatio' | 'topupGroupRatio' | null
}

function SimpleGroupDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  type,
}: SimpleGroupDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [value, setValue] = useState('')

  const title =
    type === 'groupRatio'
      ? t('systemSettings.fields.groupRatio')
      : t('systemSettings.fields.topUpRatio')

  useEffect(() => {
    if (!open) {
      setName('')
      setValue('')
      return
    }

    setName(editData?.name ?? '')
    setValue(editData?.value ?? '')
  }, [editData, open])

  const handleSave = () => {
    if (!name.trim() || !value.trim()) return
    onSave(name.trim(), value.trim())
    setName('')
    setValue('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {editData
              ? t('systemSettings.actions.editTitle', { title })
              : t('systemSettings.actions.addTitle', { title })}
          </DialogTitle>
          <DialogDescription>
            {t('systemSettings.tips.configureTheRatioForThisGroup')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('models.fields.groupName')}</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('common.fields.default')}
              disabled={!!editData}
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('dynamicRatio.fields.ratio794f65')}</Label>
            <Input
              value={value}
              onChange={(e) => {
                const val = e.target.value
                if (val === '' || !isNaN(parseFloat(val))) {
                  setValue(val)
                }
              }}
              placeholder='1.0'
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('channels.fields.update') : t('channels.actions.add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Group Override Dialog Component
type GroupOverrideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (targetGroup: string, ratio: number, oldTargetGroup?: string) => void
  editData: GroupOverride | null
  userGroup: string | null
}

function GroupOverrideDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  userGroup,
}: GroupOverrideDialogProps) {
  const { t } = useTranslation()
  const [targetGroup, setTargetGroup] = useState('')
  const [ratio, setRatio] = useState('')

  useEffect(() => {
    if (!open) {
      setTargetGroup('')
      setRatio('')
      return
    }

    setTargetGroup(editData?.targetGroup ?? '')
    setRatio(editData ? String(editData.ratio) : '')
  }, [editData, open])

  const handleSave = () => {
    if (!targetGroup.trim() || !ratio.trim()) return
    const parsedRatio = parseFloat(ratio)
    if (isNaN(parsedRatio)) return

    onSave(targetGroup.trim(), parsedRatio, editData?.targetGroup)
    setTargetGroup('')
    setRatio('')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {editData
              ? t('systemSettings.actions.editRatioOverride')
              : t('systemSettings.actions.addRatioOverride')}
          </DialogTitle>
          <DialogDescription>
            {userGroup
              ? t(
                  'systemSettings.actions.configureACustomRatioForUserGroupUsersWhenUsing',
                  { userGroup }
                )
              : t('systemSettings.tips.configureACustomRatioForWhenUsersUseA')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('systemSettings.fields.targetGroup')}</Label>
            <Input
              value={targetGroup}
              onChange={(e) => setTargetGroup(e.target.value)}
              placeholder={t('systemSettings.fields.editThis')}
              disabled={!!editData}
            />
            <p className='text-muted-foreground text-xs'>
              {t('systemSettings.tips.tokenGroupThatWillHaveACustomRatio')}
            </p>
          </div>
          <div className='space-y-2'>
            <Label>{t('dynamicRatio.fields.ratio794f65')}</Label>
            <Input
              value={ratio}
              onChange={(e) => {
                const val = e.target.value
                if (val === '' || !isNaN(parseFloat(val))) {
                  setRatio(val)
                }
              }}
              placeholder='0.9'
            />
            <p className='text-muted-foreground text-xs'>
              {t(
                'systemSettings.tips.multiplierAppliedWhenUserGroupUsesTargetGroup',
                {
                  userGroup: userGroup || t('systemSettings.fields.userGroup'),
                  targetGroup:
                    targetGroup || t('systemSettings.fields.tokenGroup48e1a2'),
                }
              )}
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.actions.cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('channels.fields.update') : t('channels.actions.add')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
