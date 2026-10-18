import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useI18n } from '../i18n'

export interface ConfirmOptions {
  title?: string
  message: string
  details?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

export interface ChoiceOption {
  label: string
  value: string
  danger?: boolean
  primary?: boolean
}

export interface ChoiceOptions {
  title?: string
  message: string
  details?: string
  options: ChoiceOption[]
  cancelText?: string
}

export interface AlertOptions {
  title?: string
  message: string
  type?: 'info' | 'error' | 'warning' | 'success'
  buttonText?: string
}

export interface ToastItem {
  id: number
  message: string
  type: 'info' | 'success' | 'error' | 'warning'
}

export const useDialogStore = defineStore('dialog', () => {
  const { t } = useI18n()

  // Confirm state
  const isConfirmOpen = ref(false)
  const confirmData = ref<ConfirmOptions>({ message: '' })
  let confirmResolver: ((val: boolean) => void) | null = null

  // Choice state
  const isChoiceOpen = ref(false)
  const choiceData = ref<ChoiceOptions>({ message: '', options: [] })
  let choiceResolver: ((val: string | null) => void) | null = null

  // Alert state
  const isAlertOpen = ref(false)
  const alertData = ref<AlertOptions>({ message: '' })
  let alertResolver: (() => void) | null = null

  // Toast state
  const toasts = ref<ToastItem[]>([])
  let nextToastId = 1

  function confirm(opts: ConfirmOptions): Promise<boolean> {
    confirmData.value = {
      title: opts.title || t('modals.confirm'),
      message: opts.message,
      details: opts.details,
      confirmText: opts.confirmText || t('modals.confirm'),
      cancelText: opts.cancelText || t('modals.cancel'),
      danger: opts.danger ?? false
    }
    isConfirmOpen.value = true
    return new Promise<boolean>((resolve) => {
      confirmResolver = resolve
    })
  }

  function handleConfirm(val: boolean) {
    isConfirmOpen.value = false
    if (confirmResolver) {
      confirmResolver(val)
      confirmResolver = null
    }
  }

  function choice(opts: ChoiceOptions): Promise<string | null> {
    choiceData.value = {
      title: opts.title || t('modals.chooseAction'),
      message: opts.message,
      details: opts.details,
      options: opts.options,
      cancelText: opts.cancelText || t('modals.cancel')
    }
    isChoiceOpen.value = true
    return new Promise<string | null>((resolve) => {
      choiceResolver = resolve
    })
  }

  function handleChoice(val: string | null) {
    isChoiceOpen.value = false
    if (choiceResolver) {
      choiceResolver(val)
      choiceResolver = null
    }
  }

  function alert(opts: AlertOptions): Promise<void> {
    alertData.value = {
      title: opts.title || (opts.type === 'error' ? t('common.error') : t('modals.info')),
      message: opts.message,
      type: opts.type || 'info',
      buttonText: opts.buttonText || t('modals.ok')
    }
    isAlertOpen.value = true
    return new Promise<void>((resolve) => {
      alertResolver = resolve
    })
  }

  function handleAlert() {
    isAlertOpen.value = false
    if (alertResolver) {
      alertResolver()
      alertResolver = null
    }
  }

  function toast(message: string, type: 'info' | 'success' | 'error' | 'warning' = 'info') {
    const id = nextToastId++
    toasts.value.push({ id, message, type })
    setTimeout(() => {
      removeToast(id)
    }, 3500)
  }

  function removeToast(id: number) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return {
    isConfirmOpen,
    confirmData,
    confirm,
    handleConfirm,

    isChoiceOpen,
    choiceData,
    choice,
    handleChoice,

    isAlertOpen,
    alertData,
    alert,
    handleAlert,

    toasts,
    toast,
    removeToast
  }
})
