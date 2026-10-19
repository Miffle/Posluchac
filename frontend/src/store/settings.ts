import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetDownloadDir, OpenDownloadFolder, GetTorrentSpeedLimits, SetTorrentSpeedLimits } from '../../wailsjs/go/main/App'

export type Language = 'ru' | 'en'

export const useSettingsStore = defineStore('settings', () => {
  const savedLang = (localStorage.getItem('posluchac_lang') as Language) || 'ru'
  const language = ref<Language>(savedLang === 'en' ? 'en' : 'ru')
  const downloadDir = ref<string>('')
  const downloadLimitKBps = ref<number>(0)
  const uploadLimitKBps = ref<number>(0)

  function setLanguage(lang: Language) {
    language.value = lang
    localStorage.setItem('posluchac_lang', lang)
  }

  async function loadDownloadDir() {
    try {
      downloadDir.value = await GetDownloadDir()
    } catch {
      // ignore
    }
  }

  async function openFolder() {
    try {
      await OpenDownloadFolder()
    } catch {
      // ignore
    }
  }

  async function loadSpeedLimits() {
    try {
      const limits = await GetTorrentSpeedLimits()
      if (limits) {
        downloadLimitKBps.value = limits.downloadKBps ?? 0
        uploadLimitKBps.value = limits.uploadKBps ?? 0
      }
    } catch {
      // ignore
    }
  }

  async function setSpeedLimits(dl: number, ul: number) {
    downloadLimitKBps.value = dl
    uploadLimitKBps.value = ul
    try {
      await SetTorrentSpeedLimits(dl, ul)
    } catch {
      // ignore
    }
  }

  return {
    language,
    downloadDir,
    downloadLimitKBps,
    uploadLimitKBps,
    setLanguage,
    loadDownloadDir,
    openFolder,
    loadSpeedLimits,
    setSpeedLimits,
  }
})
