import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  GetProgress,
  AddMagnet,
  PauseTorrent,
  ResumeTorrent,
  RemoveTorrent,
  ListTorrentFiles,
  AddFromResult,
  AddFromResultPending,
  DownloadSelectedFiles,
  CancelPendingTorrent,
  AddTorrentFileToDownload,
  RemoveTorrentFileFromDownload
} from '../../wailsjs/go/main/App'

export interface DownloadProgress {
  infoHash: string
  name: string
  downloaded: number
  total: number
  percentDone: number
  downloadSpeed: number
  uploadSpeed: number
  seeders: number
  peers?: number
  leechers?: number
  state: string
  addedAt?: number
  selectedCount?: number
  selectedTrack?: string
}

export interface TorrentFile {
  path: string
  name: string
  size: number
  infoHash: string
  bytesCompleted?: number
  percentDone?: number
  isSelected?: boolean
  isComplete?: boolean
}

export interface PendingTorrent {
  infoHash: string
  name: string
  files: TorrentFile[]
}

export const useDownloadsStore = defineStore('downloads', () => {
  const downloads = ref<DownloadProgress[]>([])
  const files = ref<Record<string, TorrentFile[]>>({}) // infoHash → files
  let pollInterval: ReturnType<typeof setInterval> | null = null

  async function refreshProgress() {
    try {
      const progress = await GetProgress()
      downloads.value = (progress || []) as DownloadProgress[]
    } catch { /* ignore */ }
  }

  function startPolling() {
    // Initial fetch immediately
    refreshProgress()

    pollInterval = setInterval(async () => {
      try {
        const progress = await GetProgress()
        downloads.value = (progress || []) as DownloadProgress[]

        // If any torrent is currently expanded, refresh its files to update per-file progress
        for (const hash of Object.keys(files.value)) {
          if (files.value[hash]) {
            await fetchFiles(hash)
          }
        }
      } catch { /* ignore */ }
    }, 1000)
  }

  function stopPolling() {
    if (pollInterval) clearInterval(pollInterval)
  }

  async function addMagnet(uri: string) {
    const res = await AddMagnet(uri)
    await refreshProgress()
    return res
  }

  async function addFromResult(result: any) {
    const res = await AddFromResult(result)
    await refreshProgress()
    return res
  }

  async function addFromResultPending(result: any): Promise<PendingTorrent> {
    const res = await AddFromResultPending(result)
    return res as PendingTorrent
  }

  async function downloadSelectedFiles(infoHash: string, selectedPaths: string[]) {
    const res = await DownloadSelectedFiles(infoHash, selectedPaths)
    await refreshProgress()
    return res
  }

  async function pause(infoHash: string) {
    await PauseTorrent(infoHash)
  }

  async function resume(infoHash: string) {
    await ResumeTorrent(infoHash)
  }

  async function remove(infoHash: string, deleteFiles: boolean) {
    await RemoveTorrent(infoHash, deleteFiles)
    delete files.value[infoHash]
  }

  async function fetchFiles(infoHash: string) {
    const list = await ListTorrentFiles(infoHash)
    files.value[infoHash] = (list || []) as TorrentFile[]
    return files.value[infoHash]
  }

  async function addFile(infoHash: string, filePath: string) {
    await AddTorrentFileToDownload(infoHash, filePath)
    await fetchFiles(infoHash)
  }

  async function cancelPendingTorrent(infoHash: string) {
    try {
      await CancelPendingTorrent(infoHash)
    } catch { /* ignore */ }
  }

  async function removeFile(infoHash: string, filePath: string) {
    await RemoveTorrentFileFromDownload(infoHash, filePath)
    await fetchFiles(infoHash)
  }

  return {
    downloads,
    files,
    refreshProgress,
    startPolling,
    stopPolling,
    addMagnet,
    addFromResult,
    addFromResultPending,
    downloadSelectedFiles,
    cancelPendingTorrent,
    pause,
    resume,
    remove,
    fetchFiles,
    addFile,
    removeFile
  }
})
