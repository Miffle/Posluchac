import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetDownloadDir, OpenDownloadFolder } from '../../wailsjs/go/main/App'

export type Language = 'ru' | 'en'

export const useSettingsStore = defineStore('settings', () => {
  const savedLang = (localStorage.getItem('posluchac_lang') as Language) || 'ru'
  const language = ref<Language>(savedLang === 'en' ? 'en' : 'ru')
  const downloadDir = ref<string>('')

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

  return {
    language,
    downloadDir,
    setLanguage,
    loadDownloadDir,
    openFolder,
  }
})
