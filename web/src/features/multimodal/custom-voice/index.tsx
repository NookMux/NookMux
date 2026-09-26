/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, version 3 of the License.
*/
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuotaWithCurrency } from '@/lib/currency'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  confirmCustomVoice,
  extractApiErrorMessage,
  getCustomVoiceConfirmQuote,
  getCustomVoiceTags,
  getTtsModels,
  previewCustomVoice,
  type CustomVoiceConfirmQuote,
  type CustomVoicePreviewResult,
} from './api'

const FALLBACK_MODELS = [
  { value: 'tts-2-hd', label: 'tts-2-hd' },
  { value: 'tts-2-turbo', label: 'tts-2-turbo' },
  { value: 'tts-1-hd', label: 'tts-1-hd' },
  { value: 'tts-1-turbo', label: 'tts-1-turbo' },
]

export function CustomVoice() {
  const { t } = useTranslation()

  const [file, setFile] = useState<File | null>(null)
  const [model, setModel] = useState('tts-2-hd')
  const [voiceId, setVoiceId] = useState('')
  const [previewText, setPreviewText] = useState('')
  const [noiseReduction, setNoiseReduction] = useState(false)
  const [volumeNormalization, setVolumeNormalization] = useState(false)

  const [previewing, setPreviewing] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [confirmQuoteLoading, setConfirmQuoteLoading] = useState(false)
  const [confirmQuote, setConfirmQuote] =
    useState<CustomVoiceConfirmQuote | null>(null)
  const [preview, setPreview] = useState<CustomVoicePreviewResult | null>(null)

  // 从后端拉取可用模型并筛选 tts- 前缀；拉取失败时回退到内置列表，保证页面可用。
  const { data: ttsModels } = useQuery({
    queryKey: ['custom-voice-tts-models'],
    queryFn: async () => {
      try {
        return await getTtsModels()
      } catch {
        return null
      }
    },
  })

  // 拉取管理员配置的情绪/语气词标签源值（后端只返回 redirect map 的 key）。
  // 仅展示用户应输入的源标签，不展示会被现有逻辑删除的上游重定向目标值。
  const { data: voiceTags } = useQuery({
    queryKey: ['custom-voice-tags'],
    queryFn: async () => {
      try {
        return await getCustomVoiceTags()
      } catch {
        return null
      }
    },
  })

  const emotionTags = voiceTags?.enabled ? (voiceTags.emotion_tags ?? []) : []
  const toneWordTags = voiceTags?.enabled
    ? (voiceTags.tone_word_tags ?? [])
    : []
  const hasTags = emotionTags.length > 0 || toneWordTags.length > 0

  const modelOptions =
    ttsModels && ttsModels.length > 0 ? ttsModels : FALLBACK_MODELS

  // 当前选中的模型不在可用列表中时，回落到第一个可用模型。
  const effectiveModel =
    modelOptions.some((m) => m.value === model) && modelOptions.length > 0
      ? model
      : (modelOptions[0]?.value ?? model)

  const validateVoiceId = (id: string) => {
    const regex = /^[a-zA-Z][a-zA-Z0-9_-]*[^-_]$/
    return id.length >= 8 && id.length <= 256 && regex.test(id)
  }

  const handlePreview = async () => {
    if (!file) {
      toast.error(t('multimodal.errors.pleaseSelectAnAudioFile'))
      return
    }
    if (!validateVoiceId(voiceId)) {
      toast.error(t('multimodal.fields.voiceIdIsInvalid'))
      return
    }
    setPreviewing(true)
    setPreview(null)
    setConfirmDialogOpen(false)
    setConfirmQuote(null)
    try {
      const res = await previewCustomVoice({
        file,
        model: effectiveModel,
        voice_id: voiceId,
        text: previewText,
        need_noise_reduction: noiseReduction,
        need_volume_normalization: volumeNormalization,
      })
      if (res.success && res.data) {
        setPreview(res.data)
        toast.success(t('multimodal.fields.previewAudioGenerated'))
      } else {
        toast.error(res.message || t('multimodal.status.previewFailed'))
      }
    } catch (e) {
      toast.error(
        extractApiErrorMessage(e) || t('multimodal.status.previewFailed')
      )
    } finally {
      setPreviewing(false)
    }
  }

  const handleConfirm = async () => {
    if (!preview) return
    setConfirming(true)
    try {
      const res = await confirmCustomVoice(preview.voice_id)
      if (res.success) {
        toast.success(t('multimodal.fields.voiceCustomizationConfirmed'))
        setConfirmDialogOpen(false)
        setConfirmQuote(null)
        setPreview(null)
        setVoiceId('')
        setFile(null)
      } else {
        toast.error(res.message || t('multimodal.status.confirmationFailed'))
      }
    } catch (e) {
      toast.error(
        extractApiErrorMessage(e) || t('multimodal.status.confirmationFailed')
      )
    } finally {
      setConfirming(false)
    }
  }

  const handleOpenConfirmDialog = async () => {
    if (!preview) return

    setConfirmQuoteLoading(true)
    setConfirmQuote(null)
    try {
      const res = await getCustomVoiceConfirmQuote(preview.voice_id)
      if (res.success && res.data) {
        setConfirmQuote(res.data)
        setConfirmDialogOpen(true)
      } else {
        toast.error(
          res.message || t('multimodal.errors.failedToFetchPaymentPrice')
        )
      }
    } catch (e) {
      toast.error(
        extractApiErrorMessage(e) ||
          t('multimodal.errors.failedToFetchPaymentPrice')
      )
    } finally {
      setConfirmQuoteLoading(false)
    }
  }

  const handleConfirmDialogOpenChange = (open: boolean) => {
    if (confirming) return
    setConfirmDialogOpen(open)
    if (!open) {
      setConfirmQuote(null)
    }
  }

  const confirmVoiceId = confirmQuote?.voice_id ?? preview?.voice_id ?? ''
  const formattedConfirmPrice = confirmQuote
    ? formatQuotaWithCurrency(confirmQuote.quota_cost)
    : '-'

  return (
    <div className='min-h-0 flex-1 space-y-6 overflow-auto'>
      <div>
        <h1 className='mb-2 text-2xl font-semibold'>
          {t('multimodal.fields.customVoice')}
        </h1>
        <p className='text-muted-foreground'>
          {t('multimodal.tips.customizeVoiceConfigurations')}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('multimodal.fields.customVoice')}</CardTitle>
          <CardDescription>
            {t('multimodal.actions.uploadAnAudioSampleChooseATtsModelEnter')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <Label>{t('multimodal.fields.audioFile')}</Label>
            <Input
              type='file'
              accept='.mp3,.m4a,.wav'
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
            <p className='text-muted-foreground text-xs'>
              {t('multimodal.tips.supportsMp3M4aWavDuration10s5minMax20')}
            </p>
          </div>

          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>{t('voiceManagement.fields.voiceId')}</Label>
              <Input
                value={voiceId}
                onChange={(e) => setVoiceId(e.target.value)}
                maxLength={256}
                placeholder='myVoice123'
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'multimodal.errors.mustStartWithALetterOnlyLettersNumbersAnd'
                )}
              </p>
            </div>

            <div className='space-y-2'>
              <Label>{t('multimodal.fields.previewModel')}</Label>
              <Select
                value={effectiveModel}
                onValueChange={(v) => v && setModel(v)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {modelOptions.map((m) => (
                    <SelectItem key={m.value} value={m.value}>
                      {m.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className='space-y-2'>
            <Label>{t('multimodal.fields.previewText')}</Label>
            <Textarea
              value={previewText}
              onChange={(e) => setPreviewText(e.target.value)}
              rows={3}
              maxLength={2000}
              placeholder={t('multimodal.fields.optionalPreviewText')}
            />
          </div>

          {voiceTags && !voiceTags.enabled && (
            <p className='text-muted-foreground text-xs'>
              {t('multimodal.status.ttsEnhancementIsDisabled')}
            </p>
          )}
          {voiceTags?.enabled && !hasTags && (
            <p className='text-muted-foreground text-xs'>
              {t('multimodal.tips.noEmotionOrToneWordTagsConfigured')}
            </p>
          )}
          {voiceTags?.enabled && hasTags && (
            <div className='space-y-2'>
              {emotionTags.length > 0 && (
                <div className='space-y-1'>
                  <p className='text-muted-foreground text-xs'>
                    {t('multimodal.fields.emotionTags')}
                  </p>
                  <div className='flex flex-wrap gap-1.5'>
                    {emotionTags.map((tag) => (
                      <Badge key={tag} variant='secondary'>
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
              {toneWordTags.length > 0 && (
                <div className='space-y-1'>
                  <p className='text-muted-foreground text-xs'>
                    {t('multimodal.tips.toneWordTagsWrapWithParentheses')}
                  </p>
                  <div className='flex flex-wrap gap-1.5'>
                    {toneWordTags.map((tag) => (
                      <Badge key={tag} variant='secondary'>
                        ({tag})
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          <div className='flex gap-6'>
            <label className='flex items-center gap-2'>
              <Checkbox
                checked={noiseReduction}
                onCheckedChange={(v) => setNoiseReduction(v === true)}
              />
              {t('multimodal.fields.noiseReduction')}
            </label>
            <label className='flex items-center gap-2'>
              <Checkbox
                checked={volumeNormalization}
                onCheckedChange={(v) => setVolumeNormalization(v === true)}
              />
              {t('multimodal.fields.volumeNormalization')}
            </label>
          </div>

          <Button onClick={handlePreview} disabled={previewing}>
            {previewing
              ? t('channels.tips.generating')
              : t('multimodal.fields.generatePreview')}
          </Button>
        </CardContent>
      </Card>

      {preview && (
        <Card>
          <CardHeader>
            <CardTitle>{t('multimodal.fields.previewAudio')}</CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            {preview.demo_audio ? (
              <audio controls className='w-full'>
                <source src={preview.demo_audio} type='audio/mpeg' />
              </audio>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('multimodal.tips.noPreviewAudioReturned')}
              </p>
            )}
            <Button
              onClick={handleOpenConfirmDialog}
              disabled={confirming || confirmQuoteLoading}
            >
              {confirmQuoteLoading && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {confirmQuoteLoading
                ? t('multimodal.status.loadingPaymentPrice')
                : t('multimodal.actions.confirmCustomization')}
            </Button>
          </CardContent>
        </Card>
      )}

      <AlertDialog
        open={confirmDialogOpen}
        onOpenChange={handleConfirmDialogOpenChange}
      >
        <AlertDialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('multimodal.actions.confirmCustomization')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'multimodal.tips.pleaseConfirmTheVoiceCustomizationDetailsAfterConfirmationTheFee'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className='space-y-3 rounded-lg border p-3 text-sm'>
            <div className='flex items-center justify-between gap-4'>
              <span className='text-muted-foreground'>
                {t('voiceManagement.fields.voiceId')}
              </span>
              <span className='font-medium break-all'>{confirmVoiceId}</span>
            </div>
            <div className='flex items-center justify-between gap-4'>
              <span className='text-muted-foreground'>
                {t('multimodal.fields.paymentPrice')}
              </span>
              <span className='font-semibold'>{formattedConfirmPrice}</span>
            </div>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={confirming}>
              {t('common.actions.cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirm}
              disabled={confirming || !confirmQuote}
            >
              {confirming && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {confirming
                ? t('multimodal.tips.confirming')
                : t('common.actions.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
