<script lang="ts" setup>
import { ref, computed, onMounted, watch } from 'vue'
import {
  GetLibrary,
  GetFavourites,
  SearchLibrary,
  ToggleFavourite,
  ScanLibrary,
  DeleteLibraryTrack
} from '../../wailsjs/go/main/App'
import { usePlayerStore } from '../store/player'
import { usePlaylistsStore } from '../store/playlists'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'
import AddToPlaylistModal from './AddToPlaylistModal.vue'
import EditMetadataModal from './EditMetadataModal.vue'

const { t, locale } = useI18n()

interface Track {
  id: number
  infoHash: string
  filePath: string
  fileName: string
  title: string
  artist: string
  album: string
  year: number
  genre: string
  durationSec: number
  sizeBytes: number
  coverPath?: string
  favourite: boolean
}

interface AlbumGroup {
  name: string
  year: number
  coverPath?: string
  tracks: Track[]
}

interface ArtistGroup {
  name: string
  tracks: Track[]
  albums: AlbumGroup[]
  coverPath?: string
  totalDuration: number
}

type Tab = 'all' | 'artists' | 'favourites'
const tab = ref<Tab>('all')
const tracks = ref<Track[]>([])
const query = ref('')
const artistFilter = ref('')
const selectedArtistName = ref('')
const loading = ref(false)

const player = usePlayerStore()
const playlistsStore = usePlaylistsStore()
const dialog = useDialogStore()
const trackForPlaylist = ref<Track | null>(null)

function openAddToPlaylist(track: Track) {
  playlistsStore.loadPlaylists()
  trackForPlaylist.value = track
}

const editingTrack = ref<Track | null>(null)

function openEditMetadata(track: Track) {
  editingTrack.value = track
}

function onMetadataSaved(updated: any) {
  if (!updated) return
  const idx = tracks.value.findIndex(t => t.id === updated.id)
  if (idx !== -1) {
    tracks.value[idx] = { ...tracks.value[idx], ...updated }
  }
}

function isTrackActive(track: Track): boolean {
  return player.status.track?.filePath === track.filePath
}

function isTrackPlaying(track: Track): boolean {
  return isTrackActive(track) && player.isPlaying
}

function handleTrackPlayClick(track: Track, list?: Track[]) {
  if (isTrackActive(track)) {
    player.togglePause()
  } else {
    playTrackInQueue(track, list)
  }
}

const props = defineProps<{
  active?: boolean
}>()

watch(() => props.active, (isActive) => {
  if (isActive) {
    loadTracks()
  }
})

async function loadTracks() {
  loading.value = true
  try {
    let res: Track[] = []
    if (tab.value === 'favourites') {
      res = ((await GetFavourites()) || []) as Track[]
      if (query.value.trim()) {
        const qWords = query.value.toLowerCase().trim().split(/\s+/)
        res = res.filter(tr => {
          const text = `${tr.title} ${tr.artist} ${tr.album} ${tr.fileName}`.toLowerCase()
          return qWords.every(w => text.includes(w))
        })
      }
    } else {
      res = (query.value.trim() ? await SearchLibrary(query.value.trim()) : await GetLibrary()) as unknown as Track[]
    }
    tracks.value = (res || []) as Track[]
  } finally {
    loading.value = false
  }
}

async function setTab(newTab: Tab) {
  tab.value = newTab
  await loadTracks()
}

async function refreshLibrary() {
  loading.value = true
  try {
    await ScanLibrary()
    await loadTracks()
  } finally {
    loading.value = false
  }
}

async function deleteTrack(track: Track) {
  const confirmed = await dialog.confirm({
    title: t('library.deleteConfirmTitle'),
    message: t('library.deleteConfirmMsg', { title: track.title || track.fileName }),
    details: t('library.deleteConfirmDetails'),
    confirmText: t('common.delete'),
    danger: true
  })
  if (!confirmed) return

  try {
    await DeleteLibraryTrack(track.id, true)
    await loadTracks()
  } catch (e: any) {
    await dialog.alert({
      title: t('common.error'),
      message: e?.message || String(e),
      type: 'error'
    })
  }
}

onMounted(async () => {
  playlistsStore.loadPlaylists()
  await ScanLibrary()
  await loadTracks()
})

async function toggleFav(track: Track) {
  await ToggleFavourite(track.id)
  track.favourite = !track.favourite
}

function playTrackInQueue(track: Track, list?: Track[]) {
  const source = list && list.length > 0 ? list : tracks.value
  const queue = source.map(item => ({
    infoHash: item.infoHash,
    filePath: item.filePath,
    fileName: item.fileName,
    title: item.title,
    artist: item.artist,
    album: item.album,
    coverPath: item.coverPath
  }))
  player.play(track.infoHash, track.filePath, queue)
}

function playArtist(artist: ArtistGroup) {
  if (!artist.tracks.length) return
  playTrackInQueue(artist.tracks[0], artist.tracks)
}

function playAlbum(albumTracks: Track[]) {
  if (!albumTracks.length) return
  playTrackInQueue(albumTracks[0], albumTracks)
}

function formatDuration(sec: number) {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

function formatDurationTotal(sec: number): string {
  if (!sec) return '0 ' + t('library.min')
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) {
    return `${h} ${t('library.hr')} ${m} ${t('library.min')}`
  }
  return `${m} ${t('library.min')}`
}

function pluralTracks(count: number): string {
  if (locale.value === 'en') {
    return count === 1 ? 'track' : 'tracks'
  }
  const n = Math.abs(count) % 100
  const n1 = n % 10
  if (n > 10 && n < 20) return 'треков'
  if (n1 > 1 && n1 < 5) return 'трека'
  if (n1 === 1) return 'трек'
  return 'треков'
}

function pluralAlbums(count: number): string {
  if (locale.value === 'en') {
    return count === 1 ? 'album' : 'albums'
  }
  const n = Math.abs(count) % 100
  const n1 = n % 10
  if (n > 10 && n < 20) return 'альбомов'
  if (n1 > 1 && n1 < 5) return 'альбома'
  if (n1 === 1) return 'альбом'
  return 'альбомов'
}

// Group tracks by artist and album
const artistsList = computed<ArtistGroup[]>(() => {
  const map = new Map<string, {
    name: string
    tracks: Track[]
    coverPath?: string
    totalDuration: number
  }>()

  for (const tr of tracks.value) {
    const rawArtist = (tr.artist || '').trim()
    const artistName = (rawArtist && rawArtist !== 'Неизвестный исполнитель') ? rawArtist : t('library.unknownArtist')
    if (!map.has(artistName)) {
      map.set(artistName, {
        name: artistName,
        tracks: [],
        coverPath: undefined,
        totalDuration: 0
      })
    }
    const a = map.get(artistName)!
    a.tracks.push(tr)
    a.totalDuration += tr.durationSec || 0
    if (!a.coverPath && tr.coverPath) {
      a.coverPath = tr.coverPath
    }
  }

  const result: ArtistGroup[] = Array.from(map.values()).map(a => {
    const albumMap = new Map<string, { name: string; year: number; coverPath?: string; tracks: Track[] }>()
    for (const tr of a.tracks) {
      const albName = (tr.album || t('library.singlesAndTracks')).trim()
      if (!albumMap.has(albName)) {
        albumMap.set(albName, {
          name: albName,
          year: tr.year || 0,
          coverPath: tr.coverPath,
          tracks: []
        })
      }
      const alb = albumMap.get(albName)!
      alb.tracks.push(tr)
      if (!alb.coverPath && tr.coverPath) {
        alb.coverPath = tr.coverPath
      }
      if (!alb.year && tr.year) {
        alb.year = tr.year
      }
    }

    const albums: AlbumGroup[] = Array.from(albumMap.values()).sort((x, y) => {
      const singlesTitle = t('library.singlesAndTracks')
      if (x.name === singlesTitle || x.name === 'Синглы и отдельные треки' || x.name === 'Singles & loose tracks') return 1
      if (y.name === singlesTitle || y.name === 'Синглы и отдельные треки' || y.name === 'Singles & loose tracks') return -1
      if (x.year && y.year && x.year !== y.year) return y.year - x.year
      return x.name.localeCompare(y.name, undefined, { sensitivity: 'base' })
    })

    return {
      name: a.name,
      tracks: a.tracks,
      albums,
      coverPath: a.coverPath,
      totalDuration: a.totalDuration
    }
  })

  result.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
  return result
})

const filteredArtists = computed(() => {
  if (!artistFilter.value.trim()) return artistsList.value
  const q = artistFilter.value.toLowerCase().trim()
  return artistsList.value.filter(a => a.name.toLowerCase().includes(q))
})

const selectedArtist = computed(() => {
  if (!filteredArtists.value.length) return null
  const found = filteredArtists.value.find(a => a.name === selectedArtistName.value)
  return found || filteredArtists.value[0] || null
})

watch(filteredArtists, (newArtists) => {
  if (!newArtists.length) {
    selectedArtistName.value = ''
    return
  }
  if (!selectedArtistName.value || !newArtists.some(a => a.name === selectedArtistName.value)) {
    selectedArtistName.value = newArtists[0].name
  }
}, { immediate: true })
</script>

<template>
  <div class="library-view">
    <div class="view-header">
      <div class="header-left">
        <h1>{{ t('library.title') }}</h1>
        <div class="tabs">
          <button :class="['tab', { active: tab === 'all' }]" @click="setTab('all')">{{ t('library.allTracks') }}</button>
          <button :class="['tab', { active: tab === 'artists' }]" @click="setTab('artists')">{{ t('library.artists') }}</button>
          <button :class="['tab', { active: tab === 'favourites' }]" @click="setTab('favourites')">❤️ {{ t('library.favourites') }}</button>
        </div>
      </div>
      <div class="header-right">
        <div class="toast-pill" v-if="playlistsStore.toastMessage">
          {{ playlistsStore.toastMessage }}
        </div>
        <button class="btn btn-ghost btn-sm scan-btn" @click="refreshLibrary" :title="t('library.rescan')">
          🔄 {{ t('library.rescan') }}
        </button>
      </div>
    </div>

    <div class="search-row">
      <input v-model="query" type="text" :placeholder="t('library.searchPlaceholder')" @input="loadTracks" />
    </div>

    <!-- Empty State -->
    <div class="empty-state" v-if="!loading && !tracks.length">
      <div class="empty-icon">🎵</div>
      <p v-if="tab === 'favourites'">{{ t('library.emptyFavourites') }}</p>
      <p v-else>{{ t('library.empty') }}</p>
      <p class="empty-hint" v-if="tab === 'favourites'">
        {{ t('library.emptyFavouritesDesc') }}
      </p>
      <p class="empty-hint" v-else>
        {{ t('library.emptyDesc') }}
      </p>
    </div>

    <!-- Tab: Artists -->
    <div class="artists-layout" v-else-if="tab === 'artists'">
      <!-- Left sidebar: artists list -->
      <div class="artists-sidebar">
        <div class="artists-search" v-if="artistsList.length > 5">
          <input v-model="artistFilter" type="text" :placeholder="t('library.filterArtists')" />
        </div>
        <div class="artists-list-header">
          <span>{{ t('library.artistsCount') }} ({{ filteredArtists.length }})</span>
        </div>
        <div class="artists-list">
          <div
            v-for="a in filteredArtists"
            :key="a.name"
            class="artist-item"
            :class="{ active: selectedArtist?.name === a.name }"
            @click="selectedArtistName = a.name"
          >
            <div class="artist-avatar">
              <img
                v-if="a.coverPath"
                :src="'/api/cover?path=' + encodeURIComponent(a.coverPath)"
                class="artist-avatar-img"
                alt="Avatar"
                @error="($event.target as HTMLElement).style.display = 'none'"
              />
              <span v-else class="artist-avatar-placeholder">👤</span>
            </div>
            <div class="artist-meta">
              <div class="artist-name-label" :title="a.name">{{ a.name }}</div>
              <div class="artist-track-count">{{ a.tracks.length }} {{ pluralTracks(a.tracks.length) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right panel: selected artist discography -->
      <div class="artist-details" v-if="selectedArtist">
        <!-- Hero header -->
        <div class="artist-hero">
          <div class="artist-hero-cover">
            <img
              v-if="selectedArtist.coverPath"
              :src="'/api/cover?path=' + encodeURIComponent(selectedArtist.coverPath)"
              class="hero-img"
              alt="Artist"
            />
            <span v-else class="hero-placeholder">👤</span>
          </div>
          <div class="artist-hero-info">
            <div class="hero-subtitle">{{ t('library.artistHeader') }}</div>
            <h2 class="hero-name">{{ selectedArtist.name }}</h2>
            <div class="hero-stats">
              <span>{{ selectedArtist.tracks.length }} {{ pluralTracks(selectedArtist.tracks.length) }}</span>
              <span class="meta-dot">·</span>
              <span>{{ formatDurationTotal(selectedArtist.totalDuration) }}</span>
              <span class="meta-dot">·</span>
              <span>{{ selectedArtist.albums.length }} {{ pluralAlbums(selectedArtist.albums.length) }}</span>
            </div>
            <div class="hero-actions">
              <button class="btn btn-accent btn-play-all" @click="playArtist(selectedArtist)">
                ▶ {{ t('library.playArtistAll') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Albums list -->
        <div class="artist-albums">
          <div
            v-for="album in selectedArtist.albums"
            :key="album.name"
            class="album-section"
          >
            <div class="album-header">
              <div class="album-cover-box">
                <img
                  v-if="album.coverPath"
                  :src="'/api/cover?path=' + encodeURIComponent(album.coverPath)"
                  class="album-cover-img"
                  alt="Album"
                  @error="($event.target as HTMLElement).style.display = 'none'"
                />
                <span v-else class="album-cover-placeholder">💿</span>
              </div>
              <div class="album-info">
                <div class="album-title">{{ album.name }}</div>
                <div class="album-meta">
                  <span v-if="album.year" class="album-year">{{ album.year }}</span>
                  <span v-if="album.year" class="meta-dot">·</span>
                  <span>{{ album.tracks.length }} {{ pluralTracks(album.tracks.length) }}</span>
                </div>
              </div>
              <button class="btn btn-ghost btn-sm album-play-btn" @click="playAlbum(album.tracks)" :title="t('library.playAlbum')">
                ▶ {{ t('library.playAlbum') }}
              </button>
            </div>

            <div class="album-track-list">
              <div
                v-for="(track, idx) in album.tracks"
                :key="track.id"
                class="track-row album-row"
                @dblclick="handleTrackPlayClick(track, selectedArtist.tracks)"
                :class="{ playing: isTrackActive(track) }"
              >
                <span class="track-num">{{ idx + 1 }}</span>
                <button
                  class="play-btn"
                  :class="{ 'is-playing': isTrackPlaying(track) }"
                  @click.stop="handleTrackPlayClick(track, selectedArtist.tracks)"
                  :title="isTrackPlaying(track) ? t('common.pause') : t('common.play')"
                >
                  {{ isTrackPlaying(track) ? '⏸' : '▶' }}
                </button>
                <div class="track-info">
                  <div class="track-title">{{ track.title || track.fileName }}</div>
                </div>
                <div class="track-genre" v-if="track.genre">{{ track.genre }}</div>
                <div class="track-duration">{{ formatDuration(track.durationSec) }}</div>
                <div class="track-actions">
                  <button
                    class="fav-btn"
                    :class="{ active: track.favourite }"
                    @click.stop="toggleFav(track)"
                    :title="track.favourite ? '❤️' : '🤍'"
                  >
                    {{ track.favourite ? '❤️' : '🤍' }}
                  </button>
                  <button class="action-btn edit-btn" @click.stop="openEditMetadata(track)" :title="t('library.editMetadata')">🏷️</button>
                  <button class="action-btn playlist-btn" @click.stop="openAddToPlaylist(track)" :title="t('library.addToPlaylist')">📋+</button>
                  <button class="del-btn" @click.stop="deleteTrack(track)" :title="t('library.deleteTrack')">🗑</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="empty-state" v-else>
        <div class="empty-icon">👥</div>
        <p>{{ t('library.noArtists') }}</p>
      </div>
    </div>

    <!-- Tab: All tracks / Favourites (flat list) -->
    <div class="all-tracks-container" v-else>
      <div class="tracks-toolbar">
        <span class="tracks-total">{{ tracks.length }} {{ pluralTracks(tracks.length) }}</span>
        <button class="btn btn-accent btn-sm" @click="playTrackInQueue(tracks[0])" v-if="tracks.length">
          ▶ {{ t('library.playAll') }}
        </button>
      </div>

      <div class="track-list">
        <div
          class="track-row flat-row"
          v-for="(track, idx) in tracks"
          :key="track.id"
          @dblclick="handleTrackPlayClick(track)"
          :class="{ playing: isTrackActive(track) }"
        >
          <span class="track-num">{{ idx + 1 }}</span>
          <button
            class="play-btn"
            :class="{ 'is-playing': isTrackPlaying(track) }"
            @click.stop="handleTrackPlayClick(track)"
            :title="isTrackPlaying(track) ? t('common.pause') : t('common.play')"
          >
            {{ isTrackPlaying(track) ? '⏸' : '▶' }}
          </button>
          <div class="track-cover-thumb-box">
            <img
              v-if="track.coverPath"
              :src="'/api/cover?path=' + encodeURIComponent(track.coverPath)"
              class="track-cover-thumb"
              alt="Cover"
              @error="($event.target as HTMLElement).style.display = 'none'"
            />
            <span v-else class="track-cover-placeholder">🎵</span>
          </div>
          <div class="track-info">
            <div class="track-title">{{ track.title || track.fileName }}</div>
            <div class="track-meta-line">
              <span class="track-artist-name">{{ (track.artist && track.artist !== 'Неизвестный исполнитель') ? track.artist : t('library.unknownArtist') }}</span>
              <span v-if="track.album" class="meta-dot">·</span>
              <span v-if="track.album" class="track-album-name">{{ track.album }}{{ track.year ? ` (${track.year})` : '' }}</span>
            </div>
          </div>
          <div class="track-genre" v-if="track.genre">{{ track.genre }}</div>
          <div class="track-duration">{{ formatDuration(track.durationSec) }}</div>
          <div class="track-actions">
            <button
              class="fav-btn"
              :class="{ active: track.favourite }"
              @click.stop="toggleFav(track)"
              :title="track.favourite ? '❤️' : '🤍'"
            >
              {{ track.favourite ? '❤️' : '🤍' }}
            </button>
            <button class="action-btn edit-btn" @click.stop="openEditMetadata(track)" :title="t('library.editMetadata')">🏷️</button>
            <button class="action-btn playlist-btn" @click.stop="openAddToPlaylist(track)" :title="t('library.addToPlaylist')">📋+</button>
            <button class="del-btn" @click.stop="deleteTrack(track)" :title="t('library.deleteTrack')">🗑</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal for adding track to playlist -->
    <AddToPlaylistModal
      :track="trackForPlaylist"
      @close="trackForPlaylist = null"
    />

    <!-- Modal for editing track metadata -->
    <EditMetadataModal
      :track="editingTrack"
      @close="editingTrack = null"
      @saved="onMetadataSaved"
    />
  </div>
</template>

<style scoped>
.library-view { display: flex; flex-direction: column; gap: 20px; }

.view-header { display: flex; justify-content: space-between; align-items: center; }
.header-left { display: flex; align-items: center; gap: 20px; }
.header-right { display: flex; align-items: center; gap: 12px; }
.view-header h1 { font-size: 24px; font-weight: 700; }
.scan-btn { display: flex; align-items: center; gap: 6px; }

.toast-pill {
  background: var(--accent);
  color: #000;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  animation: toast-in 0.15s ease-out;
}

@keyframes toast-in {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.tabs { display: flex; gap: 6px; }
.tab { padding: 7px 16px; border-radius: 20px; font-size: 13px; font-weight: 600; color: var(--text2); background: var(--surface); transition: all 0.12s; }
.tab:hover { color: var(--text); }
.tab.active { background: var(--accent); color: #000; }

.search-row input { width: 100%; }

.empty-state { text-align: center; padding: 60px 0; color: var(--text2); }
.empty-icon { font-size: 52px; margin-bottom: 14px; }
.empty-hint { font-size: 13px; margin-top: 8px; opacity: 0.7; max-width: 500px; margin-left: auto; margin-right: auto; line-height: 1.5; }
.empty-hint code { color: var(--accent); background: var(--bg3); padding: 2px 6px; border-radius: 4px; }

/* Tracks toolbar in flat view */
.tracks-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 4px 10px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 10px;
}
.tracks-total {
  font-size: 13px;
  color: var(--text2);
  font-weight: 600;
}

/* Base Track Row */
.track-list { display: flex; flex-direction: column; gap: 2px; }
.track-row {
  display: grid;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.1s;
}
.track-row:hover { background: var(--surface); }
.track-row.playing { background: var(--surface); }
.track-row.playing .track-title { color: var(--accent); }

/* Flat row in All Tracks / Favourites */
.flat-row {
  grid-template-columns: 28px 28px 36px 1fr auto auto auto;
}

/* Album row inside artist discography */
.album-row {
  grid-template-columns: 28px 28px 1fr auto auto auto;
}

.track-num {
  font-size: 13px;
  color: var(--text2);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.track-cover-thumb-box {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  overflow: hidden;
  background: var(--surface);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.track-cover-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.track-cover-placeholder {
  font-size: 18px;
  opacity: 0.7;
}



.track-info { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.track-title {
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.track-meta-line {
  font-size: 12px;
  color: var(--text2);
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.track-artist-name { color: var(--text2); font-weight: 500; }
.track-album-name { opacity: 0.8; }
.meta-dot { opacity: 0.5; font-size: 11px; }

.track-genre {
  font-size: 12px;
  color: var(--text2);
  opacity: 0.8;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.track-duration { font-size: 13px; color: var(--text2); font-variant-numeric: tabular-nums; }

.play-btn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--accent);
  color: #000;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  opacity: 0;
  transition: opacity 0.12s, transform 0.12s;
  flex-shrink: 0;
  border: none;
  cursor: pointer;
  padding: 0;
}
.track-row:hover .play-btn { opacity: 1; }
.track-row.playing .play-btn { opacity: 1; }
.play-btn:hover { transform: scale(1.12); }

.track-actions { display: flex; align-items: center; gap: 8px; }

.fav-btn {
  font-size: 14px;
  opacity: 0;
  padding: 4px;
  border-radius: 6px;
  transition: opacity 0.1s, transform 0.1s;
  background: transparent;
  cursor: pointer;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
}
.track-row:hover .fav-btn { opacity: 0.7; }
.fav-btn:hover { opacity: 1 !important; transform: scale(1.15); }
.fav-btn.active { opacity: 1 !important; }

.action-btn {
  font-size: 13px;
  opacity: 0;
  padding: 4px 6px;
  border-radius: 6px;
  transition: opacity 0.1s, background 0.1s;
  background: transparent;
  border: none;
  cursor: pointer;
  color: var(--text2);
}
.track-row:hover .action-btn { opacity: 0.8; }
.action-btn:hover { opacity: 1 !important; color: var(--accent); background: var(--bg3); }

.del-btn {
  font-size: 14px; opacity: 0; padding: 4px; border-radius: 6px;
  transition: opacity 0.1s, background 0.1s;
}
.track-row:hover .del-btn { opacity: 0.6; }
.del-btn:hover { opacity: 1 !important; background: var(--danger); }

/* ─── By Artists Tab Layout ─── */
.artists-layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 24px;
  align-items: start;
}

.artists-sidebar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 12px;
  position: sticky;
  top: 0;
  max-height: calc(100vh - 180px);
  overflow-y: auto;
}

.artists-search input {
  width: 100%;
  font-size: 13px;
  padding: 7px 10px;
}

.artists-list-header {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  color: var(--text2);
  padding: 4px 6px;
}

.artists-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.artist-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.12s;
  border-left: 3px solid transparent;
}
.artist-item:hover {
  background: var(--surface);
}
.artist-item.active {
  background: var(--surface);
  border-left-color: var(--accent);
}

.artist-avatar {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  overflow: hidden;
  background: var(--surface2);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.artist-avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.artist-avatar-placeholder {
  font-size: 18px;
  opacity: 0.7;
}

.artist-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex: 1;
}
.artist-name-label {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.artist-track-count {
  font-size: 11px;
  color: var(--text2);
  margin-top: 1px;
}

/* Selected Artist Details Pane */
.artist-details {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.artist-hero {
  display: flex;
  align-items: center;
  gap: 22px;
  background: linear-gradient(135deg, var(--surface) 0%, var(--bg2) 100%);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 20px 24px;
}
.artist-hero-cover {
  width: 96px;
  height: 96px;
  border-radius: 50%;
  overflow: hidden;
  background: var(--surface2);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.hero-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.hero-placeholder {
  font-size: 42px;
  opacity: 0.6;
}

.artist-hero-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.hero-subtitle {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 1px;
  color: var(--accent);
  margin-bottom: 4px;
}
.hero-name {
  font-size: 24px;
  font-weight: 800;
  margin-bottom: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hero-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text2);
  margin-bottom: 14px;
}
.hero-actions {
  display: flex;
  gap: 10px;
}
.btn-play-all {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  padding: 8px 18px;
  border-radius: 20px;
  font-weight: 700;
}

/* Albums */
.artist-albums {
  display: flex;
  flex-direction: column;
  gap: 22px;
}
.album-section {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
}
.album-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border);
}
.album-cover-box {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  overflow: hidden;
  background: var(--surface);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.album-cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.album-cover-placeholder {
  font-size: 20px;
  opacity: 0.7;
}
.album-info {
  flex: 1;
  min-width: 0;
}
.album-title {
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.album-meta {
  font-size: 12px;
  color: var(--text2);
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 2px;
}
.album-year {
  color: var(--accent);
  font-weight: 600;
}
.album-play-btn {
  font-size: 12px;
  padding: 6px 12px;
  border-radius: 16px;
}
.album-track-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
</style>
