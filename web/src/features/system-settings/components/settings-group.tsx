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
import {
  Children,
  createContext,
  isValidElement,
  useContext,
  useId,
  type ComponentProps,
  type ReactNode,
} from 'react'
import { useFormContext } from 'react-hook-form'
import { cn } from '@/lib/utils'
import { Checkbox } from '@/components/ui/checkbox'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

/**
 * 系统设置的统一页面结构：页面由分区组成，分区由行组成，行与行之间只用细横线分隔。
 *
 * 宽容器里行的左列是标题与说明，右列是控件与辅助信息。标题和控件同处一条 32px 高的
 * 横带并垂直居中，所以文本框、下拉与开关混排时标题始终对齐控件。输入框占满控件列；
 * 自带工具栏或表格的编辑器用 stacked 通栏，标题一行、内容一行；只放一个开关的行两端
 * 对齐，文字在左、开关贴行的右边缘，分区标题上的总开关同样贴右端。
 */

/** 行与分区标题共用的栅格底：单列，标题在上、内容在下。 */
const ROW_GRID = 'grid min-w-0 gap-x-8 gap-y-2 py-2'

/** 开关行：文字与开关同处一行、分列两端，辅助信息在下一行占满整行。 */
const TRAILING_ROW = 'flex min-w-0 flex-col gap-0 py-2'

/**
 * 两列：标签列 14rem（宽容器 16rem），其余宽度给控件列。
 * 是否分两列取决于所在容器的宽度（容器标 @container），与视口无关，
 * 所以对话框这类窄容器里自动保持单列。
 */
const TWO_COLUMN_GRID =
  '@2xl:grid-cols-[minmax(0,14rem)_minmax(0,1fr)] @6xl:grid-cols-[minmax(0,16rem)_minmax(0,1fr)]'

/**
 * 分栏里的设置面板用：标签列与控件列按 1:2 分配，面板再窄也保持两列；容器够宽时用上面的列宽。
 */
const TWO_COLUMN_ROW = `grid-cols-[minmax(0,1fr)_minmax(0,2fr)] ${TWO_COLUMN_GRID}`

/** 相邻的兄弟元素之间画一条横线，首尾不留线。 */
const RULED = '[&>*+*]:border-t'

/** 标题与控件共用的 32px 横带。 */
const BAND = 'flex min-h-8 items-center'

const TITLE_CLASS = 'text-sm leading-snug font-medium'
const DESCRIPTION_CLASS = 'text-muted-foreground pt-1 text-sm leading-snug'
/** 分区说明横跨整行，长说明不至于把头部撑高。 */
const GROUP_DESCRIPTION_CLASS =
  'text-muted-foreground col-span-2 max-w-3xl text-sm leading-snug'
const HINT_CLASS = 'text-muted-foreground pt-1 text-xs leading-snug'
const CAPTION_CLASS = 'text-muted-foreground text-xs leading-snug'
const INACTIVE_CLASS = 'text-muted-foreground'

type SettingsActivation = {
  /** 位于未激活的从属区（分组开关关闭）内。 */
  inactive: boolean
}

const SettingsActivationContext = createContext<SettingsActivation>({
  inactive: false,
})

type SettingsGroupContextValue = {
  /** 分区标题的节点 id；没有标题时为 null。 */
  titleId: string | null
}

const SettingsGroupContext = createContext<SettingsGroupContextValue>({
  titleId: null,
})

type SettingsDependentProps = ComponentProps<'fieldset'> & {
  /** 随上级开关的状态；关闭时整块禁用。 */
  enabled: boolean
}

/**
 * 从属区：缩进到一条竖线之下，关闭时内部所有控件禁用。
 * 分区带 control 时由分区自动包上；分区里的单个开关带从属行时直接使用。
 */
export function SettingsDependent({
  enabled,
  className,
  children,
  ...props
}: SettingsDependentProps) {
  return (
    <SettingsActivationContext.Provider value={{ inactive: !enabled }}>
      <fieldset
        disabled={!enabled}
        data-slot='settings-dependent'
        className={cn('ml-3 min-w-0 border-l pl-5', RULED, className)}
        {...props}
      >
        {children}
      </fieldset>
    </SettingsActivationContext.Provider>
  )
}

type SettingsGroupProps = {
  title?: ReactNode
  description?: ReactNode
  /** 分组级开关，贴标题行右端；传入后子元素视为它的从属设置。 */
  control?: ReactNode
  /** 分组级开关的状态，仅在有 control 时生效。 */
  enabled?: boolean
  /** 标题行右端的操作按钮。 */
  actions?: ReactNode
  className?: string
  children: ReactNode
}

/** 分区：标题行加一串以横线分隔的行。 */
export function SettingsGroup({
  title,
  description,
  control,
  enabled = true,
  actions,
  className,
  children,
}: SettingsGroupProps) {
  const titleId = useId()
  const hasControl = control != null
  const hasHeader =
    title != null || description != null || hasControl || actions != null

  return (
    <SettingsGroupContext.Provider
      value={{ titleId: title != null ? titleId : null }}
    >
      <section
        aria-labelledby={title != null ? titleId : undefined}
        className={cn('@container flex min-w-0 flex-col', className)}
      >
        {hasHeader && (
          <div
            className={cn(ROW_GRID, 'grid-cols-[minmax(0,1fr)_auto]', 'pb-3')}
          >
            {(title != null || hasControl || actions != null) && (
              <div className={BAND}>
                {title != null && (
                  <h3
                    id={titleId}
                    className='text-sm leading-snug font-semibold'
                  >
                    {title}
                  </h3>
                )}
              </div>
            )}
            {(hasControl || actions != null) && (
              <div className={cn(BAND, 'col-start-2 gap-2')}>
                {control}
                {actions}
              </div>
            )}
            {description != null && (
              <p className={GROUP_DESCRIPTION_CLASS}>{description}</p>
            )}
          </div>
        )}
        {hasControl ? (
          <SettingsDependent enabled={enabled}>{children}</SettingsDependent>
        ) : (
          <div className={cn('flex min-w-0 flex-col', RULED)}>{children}</div>
        )}
      </section>
    </SettingsGroupContext.Provider>
  )
}

type SettingsGroupSwitchProps =
  | {
      /** 绑定表单里的布尔字段。 */
      name: string
    }
  | {
      /** 页面自行管理状态（不在表单内）时的开关状态与切换回调。 */
      checked: boolean
      onCheckedChange: (checked: boolean) => void
    }

/** 分区标题行上的分组开关，读屏名称取分区标题：表单内绑定一个布尔字段，表单外直接受控。 */
export function SettingsGroupSwitch(props: SettingsGroupSwitchProps) {
  const form = useFormContext()
  const { titleId } = useContext(SettingsGroupContext)

  if ('checked' in props) {
    return (
      <Switch
        aria-labelledby={titleId ?? undefined}
        checked={props.checked}
        onCheckedChange={props.onCheckedChange}
      />
    )
  }

  if (form === null) {
    throw new Error('SettingsGroupSwitch 必须位于表单内。')
  }

  return (
    <FormField
      control={form.control}
      name={props.name}
      render={({ field }) => (
        <>
          <FormControl>
            <Switch
              aria-labelledby={titleId ?? undefined}
              checked={field.value === true}
              onCheckedChange={field.onChange}
            />
          </FormControl>
          <FormMessage className='text-destructive text-xs' />
        </>
      )}
    />
  )
}

/** 行里只放一个开关：直接写 Switch，或用 FormControl 包一层。 */
function isSoleSwitch(children: ReactNode): boolean {
  const items = Children.toArray(children)
  if (items.length !== 1) return false
  const only = items[0]
  if (!isValidElement<{ children?: ReactNode }>(only)) return false
  if (only.type === Switch) return true
  if (only.type !== FormControl) return false
  const inner = only.props.children
  return isValidElement(inner) && inner.type === Switch
}

type SettingsRowProps = {
  label: ReactNode
  /** 说明文字，位于标题下方。 */
  description?: ReactNode
  /** 辅助信息，位于控件下方。 */
  hint?: ReactNode
  /** 通栏：标题一行（右侧放 actions），内容占满整行。用于自带工具栏或表格的编辑器。 */
  stacked?: boolean
  /** 两列行：标签在左、控件在右，窄容器里也保持两列。用于左右分栏里的设置面板。不能与 stacked 同用。 */
  columns?: boolean
  /**
   * 两端对齐的行：文字占满剩余宽度，控件贴行的右边缘。
   * 行里只有一个开关时自动启用；开关被其他组件包住，或控件是操作按钮时传 true。
   * 不能与 stacked 同用。
   */
  trailing?: boolean
  /** 不在表单内时用来关联控件。 */
  htmlFor?: string
  /** 标题行右侧的操作按钮，配合 stacked 使用。 */
  actions?: ReactNode
  className?: string
  children: ReactNode
}

/**
 * 设置行：左列标题与说明，右列控件；位于表单内时自动显示该字段的校验错误。
 * 开关行两端对齐：标题与开关同处一行，说明在标题下方。
 * 两列行：面板再窄也保持标签在左、控件在右。
 */
export function SettingsRow({
  label,
  description,
  hint,
  stacked = false,
  columns = false,
  trailing: trailingProp,
  htmlFor,
  actions,
  className,
  children,
}: SettingsRowProps) {
  const inactive = useContext(SettingsActivationContext).inactive
  const inForm = useFormContext() !== null
  const titleClass = cn(TITLE_CLASS, inactive && INACTIVE_CLASS)
  if (stacked && trailingProp) {
    throw new Error('SettingsRow 的 stacked 与 trailing 不能同时使用。')
  }
  if (stacked && columns) {
    throw new Error('SettingsRow 的 stacked 与 columns 不能同时使用。')
  }
  const trailing = !stacked && (trailingProp ?? isSoleSwitch(children))
  const rowClass = cn(
    trailing ? TRAILING_ROW : ROW_GRID,
    !trailing && !stacked && (columns ? TWO_COLUMN_ROW : TWO_COLUMN_GRID),
    className
  )
  const descriptionClass = cn(DESCRIPTION_CLASS, stacked && 'max-w-3xl')

  const title = (
    <div
      className={cn('flex min-w-0 flex-col', trailing && 'max-w-3xl flex-1')}
    >
      <div className={cn(BAND, 'justify-between gap-3')}>
        {inForm ? (
          <FormLabel className={titleClass}>{label}</FormLabel>
        ) : htmlFor != undefined ? (
          <Label className={titleClass} htmlFor={htmlFor}>
            {label}
          </Label>
        ) : (
          <span className={titleClass}>{label}</span>
        )}
        {actions}
      </div>
      {description != null &&
        (inForm ? (
          <FormDescription className={descriptionClass}>
            {description}
          </FormDescription>
        ) : (
          <p className={descriptionClass}>{description}</p>
        ))}
    </div>
  )

  const notes = (
    <>
      {inForm && <FormMessage className='text-destructive pt-1 text-sm' />}
      {hint != null && <div className={HINT_CLASS}>{hint}</div>}
    </>
  )

  const content = trailing ? (
    <>
      <div className='flex min-w-0 items-start gap-x-8'>
        {title}
        <div className={cn(BAND, 'ml-auto')}>{children}</div>
      </div>
      {notes}
    </>
  ) : (
    <>
      {title}
      <div className='flex min-w-0 flex-col'>
        <div className={stacked ? 'flex min-w-0 flex-col' : BAND}>
          {children}
        </div>
        {notes}
      </div>
    </>
  )

  return inForm ? (
    <FormItem className={rowClass}>{content}</FormItem>
  ) : (
    <div className={rowClass}>{content}</div>
  )
}

/** 保持紧凑的选择类控件（下拉等）的最大宽度。 */
const COMPACT_CONTROL_CLASS = 'max-w-40'

type SettingsControlProps = ComponentProps<'div'> & {
  /** 下拉等选择类控件传 sm 保持紧凑；输入框默认占满控件列。 */
  size?: 'sm'
}

/** 控件列的容器：输入框占满控件列，选择类控件保持紧凑。 */
export function SettingsControl({
  size,
  className,
  ...props
}: SettingsControlProps) {
  return (
    <div
      className={cn(
        'w-full min-w-0',
        size === 'sm' && COMPACT_CONTROL_CLASS,
        className
      )}
      {...props}
    />
  )
}

const UNIT_FIELD_CLASS =
  'border-input has-disabled:bg-input/50 has-disabled:cursor-not-allowed has-disabled:opacity-50 has-[aria-invalid=true]:border-destructive has-[aria-invalid=true]:ring-destructive/20 dark:bg-input/30 dark:has-disabled:bg-input/80 dark:has-[aria-invalid=true]:border-destructive/50 dark:has-[aria-invalid=true]:ring-destructive/40 flex h-8 w-full min-w-0 items-center rounded-lg border transition-colors outline-none focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50'

const UNIT_INPUT_CLASS =
  'h-full rounded-none border-0 bg-transparent px-2.5 text-sm tabular-nums focus-visible:ring-0 disabled:bg-transparent disabled:opacity-100 aria-invalid:border-transparent aria-invalid:ring-0 dark:bg-transparent dark:disabled:bg-transparent dark:aria-invalid:border-transparent dark:aria-invalid:ring-0'

const UNIT_SELECT_TRIGGER_CLASS =
  'w-auto justify-between rounded-none border-0 bg-transparent pr-2.5 pl-2 font-normal text-muted-foreground focus-visible:ring-0 disabled:opacity-100 aria-invalid:ring-0 dark:bg-transparent dark:hover:bg-transparent'

type SettingsUnitFieldProps = ComponentProps<'div'> & {
  /** 数值输入，用 SettingsUnitInput。 */
  input: ReactNode
  /** 单位下拉，用 SettingsUnitSelect；与 unit 二选一。 */
  select?: ReactNode
  /** 固定单位的文字；与 select 二选一。 */
  unit?: ReactNode
}

const UNIT_TEXT_CLASS = 'text-muted-foreground px-2.5 text-sm'

/** 数值加单位：数值与单位拼成一个带边框的整体，占满控件列，焦点环画在外框上。 */
export function SettingsUnitField({
  input,
  select,
  unit,
  className,
  ...props
}: SettingsUnitFieldProps) {
  return (
    <div className={cn(UNIT_FIELD_CLASS, className)} {...props}>
      <div className='min-w-0 flex-1'>{input}</div>
      {(select != null || unit != null) && (
        <div className='border-input flex shrink-0 items-center border-l'>
          {select}
          {unit != null && <span className={UNIT_TEXT_CLASS}>{unit}</span>}
        </div>
      )}
    </div>
  )
}

export function SettingsUnitInput({
  className,
  ...props
}: ComponentProps<typeof Input>) {
  return <Input className={cn(UNIT_INPUT_CLASS, className)} {...props} />
}

type SettingsUnitSelectProps = {
  value: string
  onValueChange: (value: string) => void
  options: ReadonlyArray<{ value: string; label: string }>
  disabled?: boolean
  'aria-label'?: string
}

export function SettingsUnitSelect(props: SettingsUnitSelectProps) {
  return (
    <Select
      items={props.options}
      value={props.value}
      onValueChange={(value) => {
        if (value !== null) {
          props.onValueChange(value)
        }
      }}
      disabled={props.disabled}
    >
      <SelectTrigger
        className={UNIT_SELECT_TRIGGER_CLASS}
        aria-label={props['aria-label']}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {props.options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

type SettingsChoiceOption = {
  value: string
  label: ReactNode
  description?: ReactNode
}

type SettingsChoiceGridProps = ComponentProps<'div'> & {
  options: ReadonlyArray<SettingsChoiceOption>
  value: readonly string[]
  onValueChange: (value: string[]) => void
  disabled?: boolean
  columns?: 2 | 3
}

/** 多选列表：复选框加标题与说明，不逐项套卡片。 */
export function SettingsChoiceGrid({
  options,
  value,
  onValueChange,
  disabled,
  columns = 2,
  className,
  ...props
}: SettingsChoiceGridProps) {
  const inactive = useContext(SettingsActivationContext).inactive

  return (
    <div
      role='group'
      className={cn(
        'grid gap-x-6 gap-y-2.5',
        columns === 2 ? 'sm:grid-cols-2' : 'sm:grid-cols-3',
        className
      )}
      {...props}
    >
      {options.map((option) => {
        const checked = value.includes(option.value)

        return (
          <label
            key={option.value}
            className='flex cursor-pointer items-start gap-2.5 has-disabled:cursor-not-allowed'
          >
            <Checkbox
              className='mt-0.5'
              checked={checked}
              disabled={disabled}
              onCheckedChange={(next) =>
                onValueChange(
                  next === true
                    ? [...value, option.value]
                    : value.filter((item) => item !== option.value)
                )
              }
            />
            <span className='flex min-w-0 flex-col gap-0.5'>
              <span className={cn(TITLE_CLASS, inactive && INACTIVE_CLASS)}>
                {option.label}
              </span>
              {option.description != null && (
                <span className={CAPTION_CLASS}>{option.description}</span>
              )}
            </span>
          </label>
        )
      })}
    </div>
  )
}
