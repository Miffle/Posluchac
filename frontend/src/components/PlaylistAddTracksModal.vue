<script lang="ts" setup>
import { ref, computed, watch, onMounted } from 'vue'
import { GetLibrary, GetPlaylistTracks } from '../../wailsjs/go/main/App'
import { usePlaylistsStore, Playlist } from '../store/playlists'
import { useI18n } from '../i18n'

const props = defineProps<{
  playlist: Playlist | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const playlistsStore = usePlaylistsStore()
const { t } = useI18n()
const allTracks = ref<any[]>([])
const search = ref('')
const addedTrackIds = ref<Set<number>>(new Set())

async function loadData() {
  try {
    const res = await GetLibrary()
    allTracks.value = res || []
    if (props.playlist) {
      const plTracks = await GetPlaylistTracks(props.playlist.id)
      const existing = (plTracks || []).map((t: any) => t.id)
      addedTrackIds.value = new Set(existing)
    } else {
      addedTrackIds.value = new Set()
    }
  } catch (e) {
    console.error('Failed to load library or playlist tracks', e)
  }
}

watch(() => props.playlist, () => {
  loadData()
}, { immediate: true })

const filteredTracks = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return allTracks.value
  return allTracks.value.filter(t => {
    return (
      (t.title && t.title.toLowerCase().includes(q)) ||
      (t.artist && t.artist.toLowerCase().includes(q)) ||
      (t.album && t.album.toLowerCase().includes(q)) ||
      (t.fileName && t.fileName.toLowerCase().includes(q))
    )
  })
})

async function add(t: any) {
  if (!props.playlist || addedTrackIds.value.has(t.id)) return
  try {
    await playlistsStore.addTrack(props.playlist.id, t.id, props.playlist.name)
    addedTrackIds.value = new Set([...addedTrackIds.value, t.id])
  } catch (e) {
    console.error('Failed to add track to playlist', e)
  }
}

function formatDuration(sec: number) {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}
</script>

<template>
  <div class="modal-backdrop" v-if="playlist" @click.self="emit('close')">
    <div class="modal-card">
      <div class="modal-header">
        <div class="header-title">
          <span class="icon">➕</span>
          <h3>{{ t('playlists.addTracksToPlaylist', { name: playlist.name }) }}</h3>
        </div>
        <button class="close-btn" @click="emit('close')" :title="t('common.close')">✕</button>
      </div>

      <div class="search-box">
        <input
          v-model="search"
          type="text"
          :placeholder="t('playlists.searchTracksPlaceholder')"
          autofocus
        />
      </div>

      <div class="tracks-list">
        <div v-if="!filteredTracks.length" class="empty-hint">
          {{ search ? t('playlists.noTracksFound') : t('playlists.noLibraryTracks') }}
        </div>

        <div
          v-for="t in filteredTracks"
          :key="t.id"
          class="track-item"
        >
          <div class="track-meta">
            <div class="title">{{ t.title || t.fileName }}</div>
            <div class="artist">{{ t.artist || t('library.unknownArtist') }}</div>
          </div>
          <div class="duration">{{ formatDuration(t.durationSec) }}</div>
          <button
            class="btn btn-sm"
            :class="addedTrackIds.has(t.id) ? 'btn-added' : 'btn-accent'"
            @click="add(t)"
            :disabled="addedTrackIds.has(t.id)"
          >
            {{ addedTrackIds.has(t.id) ? '✓ ' + t('playlists.inPlaylist') : '+ ' + t('playlists.add') }}
          </button>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-ghost" @click="emit('close')">{{ t('playlists.done') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
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
  animation: modal-appear 0.15s ease-out;
}

@keyframes modal-appear {
  from { opacity: 0; transform: scale(0.96) translateY(6px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
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
.header-title h3 { font-size: 16px; font-weight: 700; }

.close-btn {
  font-size: 14px; color: var(--text2);
  padding: 4px 8px; border-radius: 6px; cursor: pointer;
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

.tracks-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.empty-hint {
  padding: 30px;
  text-align: center;
  color: var(--text2);
  font-size: 13px;
}

.track-item {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  transition: background 0.1s;
}
.track-item:hover { background: var(--surface); }

.track-meta { min-width: 0; }
.track-meta .title { font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.track-meta .artist { font-size: 12px; color: var(--text2); margin-top: 2px; }

.duration { font-size: 12px; color: var(--text2); font-variant-numeric: tabular-nums; }

.btn-added {
  background: rgba(29, 185, 84, 0.15);
  color: var(--accent);
  cursor: default;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  background: var(--bg3);
}
</style>
