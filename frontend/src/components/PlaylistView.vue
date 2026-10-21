<script lang="ts" setup>
import { ref, computed, onMounted } from 'vue'
import { GetLibrary, GetPlaylistTracks } from '../../wailsjs/go/main/App'
import { usePlaylistsStore, Playlist, PlaylistTrack } from '../store/playlists'
import { usePlayerStore } from '../store/player'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'
import EditMetadataModal from './EditMetadataModal.vue'

const playlistsStore = usePlaylistsStore()
const player = usePlayerStore()
const dialog = useDialogStore()
const { t } = useI18n()

const newName = ref('')
const showAddModal = ref(false)
const libraryTracks = ref<any[]>([])
const trackSearch = ref('')
const addedTrackIds = ref<Set<number>>(new Set())
const isLoadingTracks = ref(false)
const editingTrack = ref<any | null>(null)

async function openAddTracksModal() {
  if (!playlistsStore.selected) {
    if (playlistsStore.playlists.length > 0) {
      await playlistsStore.select(playlistsStore.playlists[0])
    }
  }
  if (!playlistsStore.selected) return
  showAddModal.value = true
  isLoadingTracks.value = true
  trackSearch.value = ''
  try {
    const [lib, plTracks] = await Promise.all([
      GetLibrary(),
      GetPlaylistTracks(playlistsStore.selected.id)
    ])
    libraryTracks.value = lib || []
    addedTrackIds.value = new Set((plTracks || []).map((t: any) => t.id))
  } catch (e) {
    console.error('Failed to load library or playlist tracks', e)
  } finally {
    isLoadingTracks.value = false
  }
}

const filteredLibraryTracks = computed(() => {
  const q = trackSearch.value.trim().toLowerCase()
  if (!q) return libraryTracks.value
  return libraryTracks.value.filter(t => {
    return (
      (t.title && t.title.toLowerCase().includes(q)) ||
      (t.artist && t.artist.toLowerCase().includes(q)) ||
      (t.album && t.album.toLowerCase().includes(q)) ||
      (t.fileName && t.fileName.toLowerCase().includes(q))
    )
  })
})

async function addTrackToPlaylist(track: any) {
  if (!playlistsStore.selected || addedTrackIds.value.has(track.id)) return
  try {
    await playlistsStore.addTrack(playlistsStore.selected.id, track.id, playlistsStore.selected.name)
    addedTrackIds.value = new Set([...addedTrackIds.value, track.id])
  } catch (e) {
    console.error('Failed to add track to playlist', e)
  }
}

function openEditMetadata(track: any) {
  editingTrack.value = track
}

function onMetadataSaved(updated: any) {
  if (!updated) return
  const found = playlistsStore.tracks.find(t => t.id === updated.id)
  if (found) {
    found.title = updated.title
    found.artist = updated.artist
    found.album = updated.album
    found.genre = updated.genre
  }
}

async function createPlaylist() {
  if (!newName.value.trim()) return
  await playlistsStore.create(newName.value.trim())
  newName.value = ''
}

async function deletePlaylist(id: number) {
  const ok = await dialog.confirm({
    title: t('playlists.deleteConfirmTitle'),
    message: t('playlists.deleteConfirmMsg'),
    details: t('playlists.deleteConfirmDetails'),
    confirmText: t('playlists.deletePlaylistBtn'),
    danger: true
  })
  if (!ok) return
  await playlistsStore.remove(id)
}

function formatDuration(sec: number) {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

function isTrackActive(t: PlaylistTrack): boolean {
  return player.status.track?.filePath === t.filePath
}

function isTrackPlaying(t: PlaylistTrack): boolean {
  return isTrackActive(t) && player.isPlaying
}

function handlePlayClick(t: PlaylistTrack) {
  if (isTrackActive(t)) {
    player.togglePause()
  } else {
    const queue = playlistsStore.tracks.map(item => ({
      infoHash: item.infoHash,
      filePath: item.filePath,
      fileName: item.fileName,
      title: item.title,
      artist: item.artist,
      album: item.album,
      coverPath: item.coverPath
    }))
    player.play(t.infoHash, t.filePath, queue)
  }
}

onMounted(async () => {
  await playlistsStore.loadPlaylists()
  if (!playlistsStore.selected && playlistsStore.playlists.length > 0) {
    await playlistsStore.select(playlistsStore.playlists[0])
  }
})
</script>

<template>
  <div class="playlist-view">
    <div class="view-header-row">
      <h1>{{ t('playlists.title') }}</h1>
      <div class="toast-pill" v-if="playlistsStore.toastMessage">
        {{ playlistsStore.toastMessage }}
      </div>
    </div>

    <div class="layout">
      <!-- Playlist sidebar -->
      <div class="playlist-sidebar">
        <div class="new-playlist">
          <input
            v-model="newName"
            type="text"
            :placeholder="t('playlists.namePlaceholder')"
            @keyup.enter="createPlaylist"
          />
          <button class="btn btn-accent" @click="createPlaylist" :title="t('playlists.createBtn')">+</button>
        </div>
        <button class="btn btn-import-m3u" @click="playlistsStore.importM3U" :title="t('playlists.importBtn')">
          📥 {{ t('playlists.importBtn') }}
        </button>

        <div class="playlist-items">
          <div
            v-for="p in playlistsStore.playlists"
            :key="p.id"
            class="playlist-item"
            :class="{ active: playlistsStore.selected?.id === p.id }"
            @click="playlistsStore.select(p)"
          >
            <span class="pl-icon">📋</span>
            <span class="pl-name">{{ p.name }}</span>
            <button class="pl-delete" @click.stop="deletePlaylist(p.id)" :title="t('playlists.deletePlaylistBtn')">✕</button>
          </div>
        </div>
      </div>

      <!-- Track list -->
      <div class="track-panel">
        <div v-if="!playlistsStore.selected" class="empty-state">
          <div class="empty-icon">📋</div>
          <p>{{ t('playlists.selectPlaylistHint') }}</p>
          <p class="empty-hint">{{ t('playlists.selectPlaylistSub') }}</p>
        </div>

        <template v-else>
          <div class="track-panel-header">
            <div class="header-titles">
              <h2>{{ playlistsStore.selected.name }}</h2>
              <span class="track-count">{{ playlistsStore.tracks.length }} {{ t('playlists.trackCount') }}</span>
            </div>
            <div class="header-actions">
              <button
                class="btn btn-secondary btn-sm"
                @click="playlistsStore.exportM3U(playlistsStore.selected.id)"
                :title="t('playlists.exportBtn')"
                v-if="playlistsStore.tracks.length"
              >
                💾 {{ t('playlists.exportBtn') }}
              </button>
              <button class="btn btn-accent btn-sm" @click.stop="openAddTracksModal">
                ➕ {{ t('playlists.addTracksBtn') }}
              </button>
            </div>
          </div>

          <div v-if="!playlistsStore.tracks.length" class="empty-state">
            <p class="empty-hint">{{ t('playlists.emptyPlaylist') }}</p>
            <button class="btn btn-accent btn-sm btn-empty-add" @click.stop="openAddTracksModal">
              ➕ {{ t('playlists.addFromLibrary') }}
            </button>
          </div>

          <div class="track-list" v-else>
            <div
              v-for="(track, idx) in playlistsStore.tracks"
              :key="track.id"
              class="track-row"
              @dblclick="handlePlayClick(track)"
              :class="{ playing: isTrackActive(track) }"
            >
              <span class="track-num">{{ idx + 1 }}</span>
              <div class="track-info">
                <div class="track-title">{{ track.title || track.fileName }}</div>
                <div class="track-artist" v-if="track.artist">{{ track.artist }}</div>
              </div>
              <div class="track-duration">{{ formatDuration(track.durationSec) }}</div>
              <div class="track-actions">
                <button
                  class="edit-btn"
                  @click.stop="openEditMetadata(track)"
                  :title="t('library.editMetadata')"
                >
                  🏷️
                </button>
                <button
                  class="play-btn"
                  :class="{ 'is-playing': isTrackPlaying(track) }"
                  @click.stop="handlePlayClick(track)"
                  :title="isTrackPlaying(track) ? t('common.pause') : t('common.play')"
                >
                  {{ isTrackPlaying(track) ? '⏸' : '▶' }}
                </button>
                <button
                  class="del-btn"
                  @click.stop="playlistsStore.removeTrack(playlistsStore.selected.id, track.id)"
                  :title="t('playlists.removeFromPlaylist')"
                >
                  ✕
                </button>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Inlined Modal to add library tracks into this playlist -->
    <Teleport to="body">
      <div
        class="modal-backdrop"
        v-if="showAddModal && playlistsStore.selected"
        @click.self="showAddModal = false"
      >
        <div class="modal-card">
          <div class="modal-header">
            <div class="header-title">
              <span class="icon">➕</span>
              <h3>{{ t('playlists.addTracksToPlaylist', { name: playlistsStore.selected.name }) }}</h3>
            </div>
            <button class="close-btn" @click="showAddModal = false" :title="t('common.close')">✕</button>
          </div>

          <div class="search-box">
            <input
              v-model="trackSearch"
              type="text"
              :placeholder="t('playlists.searchTracksPlaceholder')"
              autofocus
            />
          </div>

          <div class="modal-tracks-list">
            <div v-if="isLoadingTracks" class="empty-hint">
              ⏳ {{ t('common.loading') }}
            </div>
            <div v-else-if="!filteredLibraryTracks.length" class="empty-hint">
              {{ trackSearch ? t('playlists.noTracksFound') : t('playlists.noLibraryTracks') }}
            </div>
            <div
              v-else
              v-for="libTrack in filteredLibraryTracks"
              :key="libTrack.id"
              class="modal-track-item"
            >
              <div class="track-meta">
                <div class="track-title">{{ libTrack.title || libTrack.fileName }}</div>
                <div class="track-artist">{{ libTrack.artist || t('library.unknownArtist') }}</div>
              </div>
              <div class="track-duration">{{ formatDuration(libTrack.durationSec) }}</div>
              <button
                class="btn btn-sm"
                :class="addedTrackIds.has(libTrack.id) ? 'btn-added' : 'btn-accent'"
                @click="addTrackToPlaylist(libTrack)"
                :disabled="addedTrackIds.has(libTrack.id)"
              >
                {{ addedTrackIds.has(libTrack.id) ? '✓ ' + t('playlists.inPlaylist') : '+ ' + t('playlists.add') }}
              </button>
            </div>
          </div>

          <div class="modal-footer">
            <button class="btn btn-ghost" @click="showAddModal = false">{{ t('playlists.done') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Modal to edit track metadata -->
    <EditMetadataModal
      :track="editingTrack"
      @close="editingTrack = null"
      @saved="onMetadataSaved"
    />
  </div>
</template>

<style scoped>
.playlist-view { display: flex; flex-direction: column; gap: 20px; height: 100%; position: relative; }
.view-header-row { display: flex; align-items: center; justify-content: space-between; }
h1 { font-size: 24px; font-weight: 700; }

.toast-pill {
  background: var(--accent);
  color: #000;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 600;
  animation: toast-in 0.2s ease-out;
}
@keyframes toast-in {
  from { opacity: 0; transform: translateY(-6px); }
  to { opacity: 1; transform: translateY(0); }
}

.layout {
  display: grid;
  grid-template-columns: 240px 1fr;
  gap: 24px;
  flex: 1;
  min-height: 0;
}

.playlist-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.new-playlist { display: flex; gap: 8px; }
.new-playlist input { flex: 1; }
.new-playlist input:focus { border-color: var(--accent); outline: none; }
.new-playlist .btn { padding: 8px 14px; font-size: 18px; line-height: 1; }

.btn-import-m3u {
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--text);
  transition: all 0.15s ease;
}
.btn-import-m3u:hover {
  background: var(--surface2);
  border-color: var(--accent);
  color: var(--accent);
}

.playlist-items { display: flex; flex-direction: column; gap: 3px; overflow-y: auto; }
.playlist-item {
  display: flex; align-items: center; gap: 10px;
  padding: 9px 12px; border-radius: 8px;
  cursor: pointer; transition: background 0.1s;
}
.playlist-item:hover { background: var(--surface); }
.playlist-item.active { background: var(--surface2); color: var(--accent); font-weight: 600; }
.pl-icon { font-size: 14px; }
.pl-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.pl-delete {
  opacity: 0; font-size: 12px; padding: 2px 6px; border-radius: 4px;
  color: var(--text2); transition: all 0.1s;
}
.playlist-item:hover .pl-delete { opacity: 0.6; }
.pl-delete:hover { opacity: 1 !important; color: var(--danger); background: var(--bg3); }

.track-panel { display: flex; flex-direction: column; gap: 12px; overflow-y: auto; }
.track-panel-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.header-titles { display: flex; align-items: baseline; gap: 12px; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.track-panel-header h2 { font-size: 20px; font-weight: 700; }
.track-count { font-size: 13px; color: var(--text2); }

.btn-empty-add { margin-top: 10px; }

.track-list { display: flex; flex-direction: column; gap: 2px; }
.track-row {
  display: grid;
  grid-template-columns: 36px 1fr 60px 100px;
  align-items: center;
  padding: 8px 12px;
  border-radius: 6px;
  transition: background 0.1s;
}
.track-row:hover { background: var(--surface); }
.track-row.playing { color: var(--accent); }
.track-num { color: var(--text2); font-size: 13px; text-align: center; }
.track-title { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.track-artist { font-size: 12px; color: var(--text2); margin-top: 1px; }
.track-duration { color: var(--text2); font-size: 13px; text-align: right; }
.track-actions { display: flex; justify-content: flex-end; align-items: center; gap: 6px; }

.play-btn {
  opacity: 0;
  width: 28px; height: 28px;
  border-radius: 50%;
  background: var(--accent);
  color: #000;
  display: flex; align-items: center; justify-content: center;
  font-size: 11px;
  transition: all 0.1s;
}
.play-btn.is-playing { opacity: 1 !important; }
.track-row:hover .play-btn { opacity: 1; }
.track-row.playing .play-btn { opacity: 1; }
.play-btn:hover { transform: scale(1.05); }

.edit-btn {
  opacity: 0;
  font-size: 13px;
  padding: 2px 6px;
  border-radius: 4px;
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text2);
  transition: all 0.1s;
}
.track-row:hover .edit-btn { opacity: 0.8; }
.edit-btn:hover { opacity: 1 !important; color: var(--accent); background: var(--surface2); }

.del-btn {
  opacity: 0; font-size: 13px; padding: 2px 6px; border-radius: 4px;
  color: var(--text2); transition: all 0.1s;
}
.track-row:hover .del-btn { opacity: 0.6; }
.del-btn:hover { opacity: 1 !important; color: var(--danger); background: var(--surface2); }

.empty-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  height: 280px; gap: 8px; color: var(--text2); font-size: 14px;
}
.empty-icon { font-size: 40px; margin-bottom: 4px; }
.empty-hint { font-size: 12px; color: var(--text2); }

/* Inlined Modal Styles */
.modal-backdrop {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 20px;
}

.modal-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: 100%;
  max-width: 580px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.6);
  z-index: 10001;
  animation: modal-appear 0.15s ease-out;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
}
.header-title { display: flex; align-items: center; gap: 8px; }
.header-title .icon { font-size: 18px; }
.header-title h3 { font-size: 16px; font-weight: 700; color: var(--text); }

.close-btn {
  font-size: 14px;
  color: var(--text2);
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  background: transparent;
  border: none;
}
.close-btn:hover { background: var(--surface); color: var(--text); }

.search-box {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
}
.search-box input {
  width: 100%;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.search-box input:focus {
  border-color: var(--accent);
  outline: none;
}

.modal-tracks-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: calc(80vh - 140px);
}

.modal-track-item {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  transition: background 0.1s;
}
.modal-track-item:hover { background: var(--surface); }

.modal-track-item .track-meta { min-width: 0; }
.modal-track-item .track-title {
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
}
.modal-track-item .track-artist { font-size: 12px; color: var(--text2); margin-top: 2px; }
.modal-track-item .track-duration { font-size: 12px; color: var(--text2); font-variant-numeric: tabular-nums; }

.btn-added {
  background: rgba(29, 185, 84, 0.15) !important;
  color: var(--accent) !important;
  cursor: default !important;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  background: var(--bg3);
}
</style>
