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
import { memo, useCallback, useState } from 'react'
import { type UseFormReturn } from 'react-hook-form'
import { Code2, Eye, HelpCircle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField } from '@/components/ui/form'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsGroup, SettingsRow } from '../components/settings-group'
import {
  SettingsPageActionsPortal,
  SettingsPageFormActions,
} from '../components/settings-page-context'
import { GroupRatioVisualEditor } from './group-ratio-visual-editor'
import { GroupSpecialUsableRulesEditor } from './group-special-usable-editor'

type GroupFormValues = {
  GroupRatio: string
  TopupGroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  DefaultUseAutoGroup: boolean
  GroupSpecialUsableGroup: string
}

type GroupRatioFormProps = {
  form: UseFormReturn<GroupFormValues>
  onSave: (values: GroupFormValues) => Promise<void>
  isSaving: boolean
  /** 页签激活时才向页头注册操作按钮。 */
  active: boolean
}

export const GroupRatioForm = memo(function GroupRatioForm({
  form,
  onSave,
  isSaving,
  active,
}: GroupRatioFormProps) {
  const { t } = useTranslation()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [guideOpen, setGuideOpen] = useState(false)

  const handleFieldChange = useCallback(
    (field: keyof GroupFormValues, value: string) => {
      form.setValue(field, value, {
        shouldValidate: true,
        shouldDirty: true,
      })
    },
    [form]
  )

  const toggleEditMode = useCallback(() => {
    setEditMode((prev) => (prev === 'visual' ? 'json' : 'visual'))
  }, [])

  return (
    <>
      <GroupPricingGuide open={guideOpen} onOpenChange={setGuideOpen} />

      <Form {...form}>
        {active && (
          <>
            <SettingsPageActionsPortal>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => setGuideOpen(true)}
              >
                <HelpCircle data-icon='inline-start' />
                {t('systemSettings.fields.usageGuide')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={toggleEditMode}
              >
                {editMode === 'visual' ? (
                  <>
                    <Code2 data-icon='inline-start' />
                    {t('systemSettings.actions.switchToJson')}
                  </>
                ) : (
                  <>
                    <Eye data-icon='inline-start' />
                    {t('systemSettings.actions.switchToVisual')}
                  </>
                )}
              </Button>
            </SettingsPageActionsPortal>
            <SettingsPageFormActions
              onSave={form.handleSubmit(onSave)}
              isSaving={isSaving}
            />
          </>
        )}
        {editMode === 'visual' ? (
          <div className='flex min-w-0 flex-col gap-8'>
            <GroupRatioVisualEditor
              groupRatio={form.watch('GroupRatio')}
              topupGroupRatio={form.watch('TopupGroupRatio')}
              userUsableGroups={form.watch('UserUsableGroups')}
              groupGroupRatio={form.watch('GroupGroupRatio')}
              autoGroups={form.watch('AutoGroups')}
              onChange={(field, value) =>
                handleFieldChange(field as keyof GroupFormValues, value)
              }
            />

            <GroupSpecialUsableRulesEditor
              value={form.watch('GroupSpecialUsableGroup')}
              onChange={(value) =>
                handleFieldChange('GroupSpecialUsableGroup', value)
              }
            />

            <SettingsGroup>
              <FormField
                control={form.control}
                name='DefaultUseAutoGroup'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.defaultToAutoGroups')}
                    description={t(
                      'systemSettings.status.enabledNewlyCreatedTokensStartInTheFirstAuto'
                    )}
                  >
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />
            </SettingsGroup>
          </div>
        ) : (
          <SettingsForm onSubmit={form.handleSubmit(onSave)}>
            <SettingsGroup>
              <FormField
                control={form.control}
                name='GroupRatio'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.groupRatios')}
                    description={t(
                      'systemSettings.tips.jsonMapOfGroupRatioAppliedWhenTheUser'
                    )}
                  >
                    <FormControl>
                      <Textarea rows={8} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='TopupGroupRatio'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.topUpGroupRatios')}
                    description={
                      <>
                        {t(
                          'systemSettings.status.optionalMultiplierPerUserGroupUsedWhenCalculatingRecharge'
                        )}
                        {` { "default": 1, "vip": 1.2 }`}.
                      </>
                    }
                  >
                    <FormControl>
                      <Textarea rows={6} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='UserUsableGroups'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.placeholders.selectableGroups')}
                    description={t(
                      'systemSettings.tips.jsonMapOfGroupDescriptionExposedWhenUsersCreate'
                    )}
                  >
                    <FormControl>
                      <Textarea rows={6} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='GroupGroupRatio'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.interGroupOverrides')}
                    description={
                      <>
                        {t('systemSettings.fields.nestedJsonSourceGroup')}{' '}
                        {`{ targetGroup: ratio }`}{' '}
                        {t(
                          'systemSettings.tips.overrideBillingWhenAUserInOneGroupUses'
                        )}
                      </>
                    }
                  >
                    <FormControl>
                      <Textarea rows={8} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='AutoGroups'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.autoAssignmentOrder')}
                    description={t(
                      'systemSettings.status.jsonArrayOfGroupIdentifiersWhenEnabledBelowNew'
                    )}
                  >
                    <FormControl>
                      <Textarea rows={6} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='GroupSpecialUsableGroup'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.specialUsableGroupRules')}
                    description={t(
                      'systemSettings.tips.nestedJsonDefiningPerGroupRulesForAddingRemoving'
                    )}
                  >
                    <FormControl>
                      <Textarea rows={8} {...field} />
                    </FormControl>
                  </SettingsRow>
                )}
              />

              <FormField
                control={form.control}
                name='DefaultUseAutoGroup'
                render={({ field }) => (
                  <SettingsRow
                    label={t('systemSettings.fields.defaultToAutoGroups')}
                    description={t(
                      'systemSettings.status.enabledNewlyCreatedTokensStartInTheFirstAuto'
                    )}
                  >
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsRow>
                )}
              />
            </SettingsGroup>
          </SettingsForm>
        )}
      </Form>
    </>
  )
})

type GroupPricingGuideProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function GuideCodeBlock({ children }: { children: string }) {
  return (
    <pre className='bg-muted/60 overflow-x-auto rounded-lg border px-3 py-2 text-xs leading-6 whitespace-pre-wrap'>
      {children}
    </pre>
  )
}

function GroupPricingGuide({ open, onOpenChange }: GroupPricingGuideProps) {
  const { t } = useTranslation()

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-2xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t('systemSettings.fields.groupPricingUsageGuide')}
          </SheetTitle>
          <SheetDescription>
            {t(
              'systemSettings.tips.understandHowUserGroupsTokenGroupsRatiosAndSpecial'
            )}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName('gap-5')}>
          <section className='space-y-2'>
            <h3 className='text-sm font-semibold'>
              {t('systemSettings.fields.coreConcepts')}
            </h3>
            <div className='text-muted-foreground space-y-2 text-sm leading-6'>
              <p>
                <span className='text-foreground font-medium'>
                  {t('common.fields.userGroup')}
                </span>
                {': '}
                {t(
                  'systemSettings.status.assignedByAdministratorsAndUsedToRepresentAUser'
                )}
              </p>
              <p>
                <span className='text-foreground font-medium'>
                  {t('systemSettings.fields.tokenGroup')}
                </span>
                {': '}
                {t(
                  'systemSettings.placeholders.selectedWhenCreatingATokenAndUsedAsThe'
                )}
              </p>
              <p>
                <span className='text-foreground font-medium'>
                  {t('dynamicRatio.fields.ratio794f65')}
                </span>
                {': '}
                {t(
                  'systemSettings.tips.billingMultiplierLowerRatiosMeanLowerApiCallCosts'
                )}
              </p>
              <p>
                <span className='text-foreground font-medium'>
                  {t('systemSettings.fields.userSelectable')}
                </span>
                {': '}
                {t(
                  'systemSettings.status.enabledUsersCanPickThisGroupWhenCreatingTokens'
                )}
              </p>
            </div>
          </section>

          <Accordion className='rounded-lg border px-3'>
            <AccordionItem value='groups'>
              <AccordionTrigger>
                {t('systemSettings.fields.pricingGroupExample')}
              </AccordionTrigger>
              <AccordionContent className='space-y-3'>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.actions.useThePricingGroupTableToManageTheRatio'
                  )}
                </p>
                <GuideCodeBlock>
                  {`${t('models.fields.groupName')}   ${t('dynamicRatio.fields.ratio794f65')}   ${t('systemSettings.fields.userSelectable')}   ${t('auditLogs.tips.description')}
standard     1.0     ${t('systemSettings.actions.yes')}               ${t('systemSettings.fields.standardPrice')}
premium      0.5     ${t('systemSettings.actions.yes')}               ${t('systemSettings.fields.premiumPlanHalfPrice')}
vip          0.5     ${t('systemSettings.actions.no')}                ${t('systemSettings.fields.assignedByAdministratorOnly')}`}
                </GuideCodeBlock>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.tips.usersOnlySeeGroupsMarkedAsUserSelectableNon'
                  )}
                </p>
              </AccordionContent>
            </AccordionItem>

            <AccordionItem value='auto'>
              <AccordionTrigger>
                {t('systemSettings.fields.autoGroupBehavior')}
              </AccordionTrigger>
              <AccordionContent className='space-y-3'>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.tips.tokenUsesTheAutoGroupTheSystemTriesGroups'
                  )}
                </p>
                <GuideCodeBlock>{`["default", "vip"]`}</GuideCodeBlock>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.status.ifDefaultAutoGroupIsEnabledNewlyCreatedTokens'
                  )}
                </p>
              </AccordionContent>
            </AccordionItem>

            <AccordionItem value='special-ratio'>
              <AccordionTrigger>
                {t('systemSettings.fields.specialRatioRules')}
              </AccordionTrigger>
              <AccordionContent className='space-y-3'>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.tips.specialRatiosOverrideTheTokenGroupRatioForSpecific'
                  )}
                </p>
                <GuideCodeBlock>{`{
  "vip": {
    "standard": 0.8,
    "premium": 0.3
  }
}`}</GuideCodeBlock>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.tips.onlyConfiguredCombinationsAreOverriddenAllOtherCallsKeep'
                  )}
                </p>
              </AccordionContent>
            </AccordionItem>

            <AccordionItem value='usable'>
              <AccordionTrigger>
                {t('systemSettings.fields.specialUsableGroupRules')}
              </AccordionTrigger>
              <AccordionContent className='space-y-3'>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t(
                    'systemSettings.tips.specialUsableGroupRulesCanAddRemoveOrAppend'
                  )}
                </p>
                <GuideCodeBlock>{`{
  "vip": {
    "+:premium": "${t('systemSettings.fields.premiumPlanHalfPrice')}",
    "-:default": "remove",
    "special": "${t('systemSettings.fields.specialGroup')}"
  }
}`}</GuideCodeBlock>
                <p className='text-muted-foreground text-sm leading-6'>
                  {t('systemSettings.actions.useToAddAGroupToRemoveADefault')}
                </p>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </div>
      </SheetContent>
    </Sheet>
  )
}
