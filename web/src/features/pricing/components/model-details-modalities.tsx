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
  FileText,
  Image as ImageIcon,
  Mic2,
  Type as TypeIcon,
  Video,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { Modality } from '../types'

type IconComponent = React.ComponentType<{ className?: string }>

const MODALITY_META: Record<
  Modality,
  { icon: IconComponent; labelKey: string }
> = {
  text: { icon: TypeIcon, labelKey: 'common.fields.text' },
  image: { icon: ImageIcon, labelKey: 'models.fields.image' },
  audio: { icon: Mic2, labelKey: 'common.fields.audio' },
  video: { icon: Video, labelKey: 'pricing.fields.video' },
  file: { icon: FileText, labelKey: 'common.fields.file' },
}

/** Inline modality icons (used by the quick-stats flow). */
export function ModalityIcons(props: {
  modalities: Modality[]
  className?: string
}) {
  const { t } = useTranslation()
  if (props.modalities.length === 0) {
    return <span className='text-muted-foreground text-xs'>—</span>
  }
  return (
    <span className='inline-flex items-center gap-1'>
      {props.modalities.map((modality) => {
        const meta = MODALITY_META[modality]
        const Icon = meta.icon
        return (
          <Tooltip key={modality}>
            <TooltipTrigger
              render={
                <span
                  aria-label={t(meta.labelKey)}
                  className='text-foreground/80 inline-flex'
                />
              }
            >
              <Icon className={cn('size-3.5', props.className)} />
            </TooltipTrigger>
            <TooltipContent side='top' className='text-xs'>
              {t(meta.labelKey)}
            </TooltipContent>
          </Tooltip>
        )
      })}
    </span>
  )
}
