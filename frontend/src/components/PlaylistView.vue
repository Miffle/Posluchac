<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { ListPlaylists, CreatePlaylist, DeletePlaylist, GetPlaylistTracks, AddToPlaylist } from '../../wailsjs/go/main/App'
import { usePlayerStore } from '../store/player'

interface Playlist { id: number; name: string; createdAt: string }
interface Track { id: number; infoHash: string; filePath: string; fileName: string; title: string; artist: string; durationSec: number }

const playlists = ref<Playlist[]>([])
const selected = ref<Playlist | null>(null)
const tracks = ref<Track[]>([])
const newName = ref('')
const player = usePlayerStore()

async function load() {
  const res = await ListPlaylists()
  playlists.value = (res || []) as Playlist[]
}

async function selectPlaylist(p: Playlist) {
  selected.value = p
  const res = await GetPlaylistTracks(p.id)
  tracks.value = (res || []) as Track[]
}

async function createPlaylist() {
  if (!newName.value.trim()) return
  await CreatePlaylist(newName.value.trim())
  newName.value = ''
  await load()
}

async function deletePlaylist(id: number) {
  if (!confirm('Удалить плейлист?')) return
  await DeletePlaylist(id)
  if (selected.value?.id === id) { selected.value = null; tracks.value = [] }
  await load()
}

function formatDuration(sec: number) {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

onMounted(load)
</script>

<template>
  <div class="playlist-view">
    <h1>Плейлисты</h1>

    <div class="layout">
      <!-- Playlist list -->
      <div class="playlist-sidebar">
        <div class="new-playlist">
          <input v-model="newName" type="text" placeholder="Новый плейлист..." @keyup.enter="createPlaylist" />
          <button class="btn btn-accent" @click="createPlaylist">+</button>
        </div>

        <div class="playlist-items">
          <div
            v-for="p in playlists"
            :key="p.id"
            class="playlist-item"
            :class="{ active: selected?.id === p.id }"
            @click="selectPlaylist(p)"
          >
            <span class="pl-icon">📋</span>
            <span class="pl-name">{{ p.name }}</span>
            <button class="pl-delete" @click.stop="deletePlaylist(p.id)" title="Удалить">✕</button>
          </div>
        </div>
      </div>

      <!-- Track list -->
      <div class="track-panel">
        <div v-if="!selected" class="empty-state">
          <div class="empty-icon">📋</div>
          <p>Выбери плейлист</p>
        </div>

        <template v-else>
          <div class="track-panel-header">
            <h2>{{ selected.name }}</h2>
            <span class="track-count">{{ tracks.length }} треков</span>
          </div>

          <div v-if="!tracks.length" class="empty-state">
            <p class="empty-hint">Плейлист пуст. Добавь треки из библиотеки.</p>
          </div>

          <div class="track-list" v-else>
            <div
              v-for="(t, idx) in tracks"
              :key="t.id"
              class="track-row"
              @dblclick="player.play(t.infoHash, t.filePath)"
              :class="{ playing: player.status.track?.filePath === t.filePath }"
            >
              <span class="track-num">{{ idx + 1 }}</span>
              <div class="track-info">
                <div class="track-title">{{ t.title || t.fileName }}</div>
                <div class="track-artist" v-if="t.artist">{{ t.artist }}</div>
              </div>
              <div class="track-duration">{{ formatDuration(t.durationSec) }}</div>
              <button class="play-btn" @click="player.play(t.infoHash, t.filePath)">▶</button>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.playlist-view { display: flex; flex-direction: column; gap: 20px; height: 100%; }
h1 { font-size: 24px; font-weight: 700; }

.layout { display: grid; grid-template-columns: 240px 1fr; gap: 20px; flex: 1; min-height: 0; }

.playlist-sidebar { display: flex; flex-direction: column; gap: 12px; }
.new-playlist { display: flex; gap: 8px; }
.new-playlist input { flex: 1; }
.new-playlist .btn { padding: 8px 14px; font-size: 18px; line-height: 1; }

.playlist-items { display: flex; flex-direction: column; gap: 3px; overflow-y: auto; }
.playlist-item {
  display: flex; align-items: center; gap: 10px;
  padding: 9px 12px; border-radius: 8px;
  cursor: pointer; transition: background 0.1s;
}
.playlist-item:hover { background: var(--surface); }
.playlist-item.active { background: var(--surface); color: var(--accent); }
.pl-icon { font-size: 15px; }
.pl-name { flex: 1; font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pl-delete { opacity: 0; font-size: 13px; padding: 2px 6px; border-radius: 4px; transition: opacity 0.1s, background 0.1s; }
.playlist-item:hover .pl-delete { opacity: 0.5; }
.pl-delete:hover { opacity: 1 !important; background: var(--danger); color: #fff; }

/* Track panel */
.track-panel { display: flex; flex-direction: column; gap: 12px; overflow-y: auto; }
.track-panel-header { display: flex; align-items: baseline; gap: 12px; }
.track-panel-header h2 { font-size: 20px; font-weight: 700; }
.track-count { font-size: 13px; color: var(--text2); }

.empty-state { text-align: center; padding: 40px 0; color: var(--text2); }
.empty-icon { font-size: 48px; margin-bottom: 12px; }
.empty-hint { font-size: 13px; opacity: 0.7; }

.track-list { display: flex; flex-direction: column; }
.track-row {
  display: grid;
  grid-template-columns: 28px 1fr auto auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.1s;
}
.track-row:hover { background: var(--surface); }
.track-row.playing .track-title { color: var(--accent); }
.track-num { font-size: 13px; color: var(--text2); text-align: right; }
.track-title { font-size: 14px; font-weight: 500; }
.track-artist { font-size: 12px; color: var(--text2); margin-top: 1px; }
.track-duration { font-size: 13px; color: var(--text2); font-variant-numeric: tabular-nums; }
.play-btn {
  font-size: 12px; padding: 4px 12px;
  background: var(--accent); color: #000;
  border-radius: 12px; font-weight: 700;
  opacity: 0; transition: opacity 0.1s;
}
.track-row:hover .play-btn { opacity: 1; }
</style>
