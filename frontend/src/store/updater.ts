import { defineStore } from 'pinia'
import { ref } from 'vue'
import { CheckForUpdates, ApplyUpdate, OpenExternalURL, GetAppVersion } from '../../wailsjs/go/main/App'
import { updater } from '../../wailsjs/go/models'

export const useUpdaterStore = defineStore('updater', () => {
  const currentVersion = ref('')
  const updateInfo = ref<updater.UpdateInfo | null>(null)
  const isChecking = ref(false)
  const isUpdating = ref(false)
  const showModal = ref(false)
  const error = ref<string | null>(null)
  const statusMessage = ref<string | null>(null)

  async function init() {
    try {
      currentVersion.value = await GetAppVersion()
    } catch { /* ignore */ }
    // Auto check silently in background after 3 seconds
    setTimeout(() => {
      check(false)
    }, 3000)
  }

  async function check(manual = true) {
    if (isChecking.value) return
    isChecking.value = true
    error.value = null
    try {
      const res = await CheckForUpdates()
      updateInfo.value = res
      if (res && res.hasUpdate) {
        if (manual) {
          showModal.value = true
        }
      } else if (manual) {
        statusMessage.value = 'У вас установлена актуальная версия!'
        setTimeout(() => { statusMessage.value = null }, 3500)
      }
    } catch (e: any) {
      if (manual) {
        error.value = 'Ошибка проверки: ' + (e?.message || e)
        setTimeout(() => { error.value = null }, 4000)
      }
    } finally {
      isChecking.value = false
    }
  }

  async function apply() {
    if (!updateInfo.value?.downloadUrl) return
    isUpdating.value = true
    error.value = null
    try {
      await ApplyUpdate(updateInfo.value.downloadUrl)
    } catch (e: any) {
      isUpdating.value = false
      error.value = 'Не удалось применить обновление: ' + (e?.message || e)
    }
  }

  async function openRelease() {
    if (updateInfo.value?.releaseUrl) {
      await OpenExternalURL(updateInfo.value.releaseUrl)
    }
  }

  return {
    currentVersion,
    updateInfo,
    isChecking,
    isUpdating,
    showModal,
    error,
    statusMessage,
    init,
    check,
    apply,
    openRelease,
  }
})
