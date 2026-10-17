<script lang="ts" setup>
import { ref, watch } from 'vue'
import { usePlaylistsStore, Playlist } from '../store/playlists'

const props = defineProps<{
  track: any | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const playlistsStore = usePlaylistsStore()
const newPlaylistName = ref('')
const addingToId = ref<number | null>(null)
const existingPlaylistIds = ref<Set<number>>(new Set())
const loadingIds = ref(false)

watch(() => props.track, async (t) => {
  if (t) {
    loadingIds.value = true
    try {
      const ids = await playlistsStore.getPlaylistsForTrack(t.id)
      existingPlaylistIds.value = new Set(ids)
    } finally {
      loadingIds.value = false
    }
  } else {
    existingPlaylistIds.value = new Set()
  }
}, { immediate: true })

async function handleCreateAndAdd() {
  const name = newPlaylistName.value.trim()
  if (!name || !props.track) return
  const newId = await playlistsStore.create(name)
  newPlaylistName.value = ''
  if (newId) {
    await playlistsStore.addTrack(newId, props.track.id, name)
    existingPlaylistIds.value.add(newId)
  }
}

async function handleToggle(p: Playlist) {
  if (!props.track || addingToId.value !== null) return
  addingToId.value = p.id
  try {
    if (existingPlaylistIds.value.has(p.id)) {
      await playlistsStore.removeTrack(p.id, props.track.id, p.name)
      existingPlaylistIds.value.delete(p.id)
    } else {
      await playlistsStore.addTrack(p.id, props.track.id, p.name)
      existingPlaylistIds.value.add(p.id)
    }
  } finally {
    addingToId.value = null
  }
}
</script>

<template>
  <div class="modal-backdrop" v-if="track" @click.self="emit('close')">
    <div class="modal-card">
      <div class="modal-header">
        <div class="header-title">
          <span class="icon">📋</span>
          <h3>Добавить в плейлист</h3>
        </div>
        <button class="close-btn" @click="emit('close')" title="Закрыть">✕</button>
      </div>

      <div class="track-summary">
        <div class="summary-title">{{ track.title || track.fileName }}</div>
        <div class="summary-artist" v-if="track.artist">{{ track.artist }}</div>
      </div>

      <!-- Quick create new playlist -->
      <div class="create-row">
        <input
          v-model="newPlaylistName"
          type="text"
          placeholder="Создать новый плейлист..."
          @keyup.enter="handleCreateAndAdd"
        />
        <button
          class="btn btn-accent btn-sm"
          @click="handleCreateAndAdd"
          :disabled="!newPlaylistName.trim()"
        >
          Создать
        </button>
      </div>

      <!-- Playlists list -->
      <div class="playlists-list">
        <div v-if="loadingIds" class="empty-hint">
          Проверка плейлистов...
        </div>

        <div v-else-if="!playlistsStore.playlists.length" class="empty-hint">
          Плейлистов пока нет. Введите название выше и создайте первый!
        </div>

        <div
          v-for="p in playlistsStore.playlists"
          :key="p.id"
          class="playlist-option"
          :class="{ 'in-playlist': existingPlaylistIds.has(p.id) }"
          @click="handleToggle(p)"
        >
          <div class="playlist-info">
            <span class="pl-icon">{{ existingPlaylistIds.has(p.id) ? '✅' : '🎵' }}</span>
            <span class="pl-name">{{ p.name }}</span>
          </div>
          <button
            class="btn btn-sm action-badge-btn"
            :class="existingPlaylistIds.has(p.id) ? 'btn-added' : 'btn-ghost add-btn'"
          >
            <span v-if="addingToId === p.id">...</span>
            <span v-else-if="existingPlaylistIds.has(p.id)">✓ В плейлисте</span>
            <span v-else>+ Добавить</span>
          </button>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-accent btn-sm" @click="emit('close')">Готово</button>
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
  max-width: 440px;
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

.header-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.header-title .icon { font-size: 18px; }
.header-title h3 { font-size: 16px; font-weight: 700; }

.close-btn {
  font-size: 14px;
  color: var(--text2);
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.1s;
}
.close-btn:hover { background: var(--surface); color: var(--text); }

.track-summary {
  padding: 12px 20px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}
.summary-title {
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.summary-artist {
  font-size: 12px;
  color: var(--text2);
  margin-top: 2px;
}

.create-row {
  display: flex;
  gap: 8px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border);
}
.create-row input {
  flex: 1;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 7px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.create-row input:focus {
  border-color: var(--accent);
  outline: none;
}

.playlists-list {
  padding: 8px 12px;
  max-height: 260px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.empty-hint {
  padding: 20px 10px;
  text-align: center;
  font-size: 13px;
  color: var(--text2);
}

.playlist-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.12s;
}
.playlist-option:hover {
  background: var(--surface);
}

.playlist-info {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.pl-icon { font-size: 16px; opacity: 0.8; }
.pl-name {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.action-badge-btn {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 6px;
}

.add-btn {
  color: var(--accent);
  background: rgba(29, 185, 84, 0.1);
}
.playlist-option:hover .add-btn {
  background: var(--accent);
  color: #000;
}

.btn-added {
  background: rgba(29, 185, 84, 0.18);
  color: var(--accent);
  border: 1px solid rgba(29, 185, 84, 0.4);
  font-weight: 600;
  transition: all 0.15s;
}
.playlist-option:hover .btn-added {
  background: rgba(220, 53, 69, 0.15);
  color: #ff6b6b;
  border-color: rgba(220, 53, 69, 0.4);
}
.playlist-option:hover .btn-added::after {
  content: ' (убрать)';
}

.modal-footer {
  padding: 10px 16px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  background: var(--bg2);
}
</style>
