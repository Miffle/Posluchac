import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useI18n } from '../i18n'
import {
  ListPlaylists,
  CreatePlaylist,
  DeletePlaylist,
  GetPlaylistTracks,
  AddToPlaylist,
  RemoveFromPlaylist,
  GetPlaylistsForTrack,
  ImportPlaylist,
  ExportPlaylist
} from '../../wailsjs/go/main/App'

export interface Playlist {
  id: number
  name: string
  createdAt: string
}

export interface PlaylistTrack {
  id: number
  infoHash: string
  filePath: string
  fileName: string
  title: string
  artist: string
  album?: string
  genre?: string
  durationSec: number
  coverPath?: string
  favourite?: boolean
}

export const usePlaylistsStore = defineStore('playlists', () => {
  const { t } = useI18n()
  const playlists = ref<Playlist[]>([])
  const selected = ref<Playlist | null>(null)
  const tracks = ref<PlaylistTrack[]>([])
  const toastMessage = ref<string | null>(null)

  function showToast(msg: string) {
    toastMessage.value = msg
    setTimeout(() => {
      if (toastMessage.value === msg) {
        toastMessage.value = null
      }
    }, 2800)
  }

  async function loadPlaylists() {
    try {
      const res = await ListPlaylists()
      playlists.value = (res || []) as Playlist[]
    } catch (e) {
      console.error('Failed to list playlists', e)
    }
  }

  async function create(name: string): Promise<number> {
    const trimmed = name.trim()
    if (!trimmed) return 0
    const id = await CreatePlaylist(trimmed)
    await loadPlaylists()
    const created = playlists.value.find(p => p.id === id)
    if (created) {
      await select(created)
    }
    showToast(t('playlists.toastCreatedWithName', { name: trimmed }))
    return id
  }

  async function remove(id: number) {
    await DeletePlaylist(id)
    if (selected.value?.id === id) {
      selected.value = null
      tracks.value = []
    }
    await loadPlaylists()
  }

  async function select(p: Playlist) {
    selected.value = p
    await refreshSelectedTracks()
  }

  async function refreshSelectedTracks() {
    if (!selected.value) return
    try {
      const res = await GetPlaylistTracks(selected.value.id)
      tracks.value = (res || []) as PlaylistTrack[]
    } catch (e) {
      console.error('Failed to get playlist tracks', e)
    }
  }

  async function addTrack(playlistId: number, trackId: number, playlistName?: string) {
    await AddToPlaylist(playlistId, trackId, 0)
    if (selected.value?.id === playlistId) {
      await refreshSelectedTracks()
    }
    const name = playlistName || playlists.value.find(p => p.id === playlistId)?.name || ''
    showToast(t('playlists.toastAdded', { name }))
  }

  async function removeTrack(playlistId: number, trackId: number, playlistName?: string) {
    await RemoveFromPlaylist(playlistId, trackId)
    if (selected.value?.id === playlistId) {
      await refreshSelectedTracks()
    }
    const name = playlistName || playlists.value.find(p => p.id === playlistId)?.name || ''
    showToast(t('playlists.toastRemoved', { name }))
  }

  async function getPlaylistsForTrack(trackId: number): Promise<number[]> {
    try {
      const res = await GetPlaylistsForTrack(trackId)
      return (res || []) as number[]
    } catch (e) {
      console.error('Failed to get playlists for track', e)
      return []
    }
  }

  async function importM3U(): Promise<Playlist | null> {
    try {
      const imported = await ImportPlaylist()
      if (imported) {
        await loadPlaylists()
        await select(imported as Playlist)
        showToast(t('playlists.toastImportedWithName', { name: imported.name }))
        return imported as Playlist
      }
    } catch (e) {
      console.error('Failed to import playlist', e)
    }
    return null
  }

  async function exportM3U(id: number): Promise<string | null> {
    try {
      const path = await ExportPlaylist(id)
      if (path) {
        showToast(t('playlists.toastExported'))
        return path
      }
    } catch (e) {
      console.error('Failed to export playlist', e)
    }
    return null
  }

  return {
    playlists,
    selected,
    tracks,
    toastMessage,
    loadPlaylists,
    create,
    remove,
    select,
    refreshSelectedTracks,
    addTrack,
    removeTrack,
    getPlaylistsForTrack,
    showToast,
    importM3U,
    exportM3U
  }
})
