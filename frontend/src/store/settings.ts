import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  GetDownloadDir,
  OpenDownloadFolder,
  GetTorrentSpeedLimits,
  SetTorrentSpeedLimits,
  GetLibraryFolders,
  AddLibraryFolder,
  RemoveLibraryFolder,
  ChooseLibraryFolderDialog,
  ScanLibrary
} from '../../wailsjs/go/main/App'

export type Language = 'ru' | 'en'

export interface LibraryFolder {
  id: number
  path: string
  isDefault: boolean
  addedAt?: string
}

export const useSettingsStore = defineStore('settings', () => {
  const savedLang = (localStorage.getItem('posluchac_lang') as Language) || 'ru'
  const language = ref<Language>(savedLang === 'en' ? 'en' : 'ru')
  const downloadDir = ref<string>('')
  const downloadLimitKBps = ref<number>(0)
  const uploadLimitKBps = ref<number>(0)
  const libraryFolders = ref<LibraryFolder[]>([])
  const isScanning = ref<boolean>(false)

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

  async function loadLibraryFolders() {
    try {
      const res = await GetLibraryFolders()
      libraryFolders.value = (res || []) as LibraryFolder[]
    } catch {
      // ignore
    }
  }

  async function addFolderDialog(): Promise<boolean> {
    try {
      const path = await ChooseLibraryFolderDialog()
      if (!path) return false
      await AddLibraryFolder(path)
      await loadLibraryFolders()
      return true
    } catch {
      return false
    }
  }

  async function removeFolder(id: number, removeTracks: boolean) {
    try {
      await RemoveLibraryFolder(id, removeTracks)
      await loadLibraryFolders()
    } catch {
      // ignore
    }
  }

  async function rescanLibrary(): Promise<number> {
    isScanning.value = true
    try {
      const count = await ScanLibrary()
      return count
    } finally {
      isScanning.value = false
    }
  }

  return {
    language,
    downloadDir,
    downloadLimitKBps,
    uploadLimitKBps,
    libraryFolders,
    isScanning,
    setLanguage,
    loadDownloadDir,
    openFolder,
    loadSpeedLimits,
    setSpeedLimits,
    loadLibraryFolders,
    addFolderDialog,
    removeFolder,
    rescanLibrary,
  }
})
