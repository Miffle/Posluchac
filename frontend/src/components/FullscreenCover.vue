<script lang="ts" setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { usePlayerStore } from '../store/player'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'
import { GetLyrics } from '../../wailsjs/go/main/App'
import { parseLRC, type LyricLine } from '../utils/lrc'

const player = usePlayerStore()
const dialog = useDialogStore()
const { t } = useI18n()
const s = computed(() => player.status)

async function handleSaveTrack() {
  if (player.isCurrentSaved || player.isSavingTrack) return
  try {
    const ok = await player.saveCurrentTrack()
    if (ok) {
      dialog.toast(t('player.toastTrackSaved'), 'success')
    }
  } catch (err: any) {
    dialog.toast(err?.message || String(err), 'error')
  }
}

const showLyrics = ref<boolean>(true)

// Lyrics data state
const isLoadingLyrics = ref<boolean>(false)
const lyricsError = ref<string>('')
const lyricsData = ref<{
  found: boolean
  artist: string
  title: string
  album: string
  duration: number
  instrumental: boolean
  plainLyrics: string
  syncedLyrics: string
  source: string
} | null>(null)

const lastLoadedKey = ref<string>('')
const lyricsContainerRef = ref<HTMLElement | null>(null)
// Only set to true when user physically scrolls with wheel, touch, or drag
const isUserScrolled = ref<boolean>(false)

const coverUrl = computed(() => {
  if (s.value.track?.coverPath) {
    return '/api/cover?path=' + encodeURIComponent(s.value.track.coverPath)
  }
  if (player.currentQueueTrack?.coverPath) {
    return '/api/cover?path=' + encodeURIComponent(player.currentQueueTrack.coverPath)
  }
  return ''
})

const displayTitle = computed(() => {
  if (s.value.track?.title) return s.value.track.title
  if (player.currentQueueTrack?.title) return player.currentQueueTrack.title
  const fn = s.value.track?.fileName || player.currentQueueTrack?.fileName || ''
  if (!fn) return t('player.noTrack')
  const nameWithoutExt = fn.replace(/\.[^/.]+$/, '')
  if (nameWithoutExt.includes(' - ')) {
    return nameWithoutExt.split(' - ').slice(1).join(' - ').trim()
  }
  return nameWithoutExt
})

const displayArtist = computed(() => {
  if (s.value.track?.artist) return s.value.track.artist
  if (player.currentQueueTrack?.artist) return player.currentQueueTrack.artist
  const fn = s.value.track?.fileName || player.currentQueueTrack?.fileName || ''
  if (!fn) return ''
  const nameWithoutExt = fn.replace(/\.[^/.]+$/, '')
  if (nameWithoutExt.includes(' - ')) {
    return nameWithoutExt.split(' - ')[0].trim()
  }
  return ''
})

const displayAlbum = computed(() => {
  if (s.value.track?.album) return s.value.track.album
  if (player.currentQueueTrack?.album) return player.currentQueueTrack.album
  return ''
})

const syncedLines = computed<LyricLine[]>(() => {
  if (!lyricsData.value || !lyricsData.value.syncedLyrics) return []
  return parseLRC(lyricsData.value.syncedLyrics)
})

const plainLines = computed<string[]>(() => {
  if (!lyricsData.value || !lyricsData.value.plainLyrics) return []
  return lyricsData.value.plainLyrics.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0)
})

// Current active line index in syncedLyrics
const activeLineIndex = computed(() => {
  const lines = syncedLines.value
  if (lines.length === 0) return -1
  const pos = s.value.positionSec || 0

  let active = -1
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].time <= pos + 0.15) {
      active = i
    } else {
      break
    }
  }

  // If at beginning of song before line 0, show line 0 as active
  if (active === -1 && lines.length > 0 && pos < (lines[0].time || 5)) {
    return 0
  }
  return active
})

// Auto-scroll active line directly to the vertical center of the container
function scrollToActiveLine(behavior: ScrollBehavior = 'smooth') {
  const container = lyricsContainerRef.value
  if (!container || activeLineIndex.value < 0) return

  const el = container.querySelector(`[data-index="${activeLineIndex.value}"]`) as HTMLElement
  if (!el) return

  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()

  // Calculate delta between element's current vertical center and container's vertical center
  const currentCenter = elRect.top + elRect.height / 2
  const targetCenter = containerRect.top + containerRect.height / 2
  const delta = currentCenter - targetCenter

  if (Math.abs(delta) > 1.5) {
    container.scrollTo({
      top: container.scrollTop + delta,
      behavior
    })
  }
}

// Watch active line: if user hasn't manually scrolled away, always keep active line in center
watch(activeLineIndex, () => {
  if (!isUserScrolled.value) {
    scrollToActiveLine('smooth')
  }
})

// Detect physical user manual scrolling (wheel, touch, or drag)
function onUserScrollInteraction() {
  if (!isUserScrolled.value) {
    isUserScrolled.value = true
  }
}

// If user scrolls back close to the active line on their own, re-enable auto-center
function onContainerScroll() {
  if (!isUserScrolled.value) return
  const container = lyricsContainerRef.value
  if (!container || activeLineIndex.value < 0) return
  const el = container.querySelector(`[data-index="${activeLineIndex.value}"]`) as HTMLElement
  if (!el) return

  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()
  const currentCenter = elRect.top + elRect.height / 2
  const targetCenter = containerRect.top + containerRect.height / 2
  const delta = Math.abs(currentCenter - targetCenter)

  // Within 35px of center: user scrolled back to current line
  if (delta < 35) {
    isUserScrolled.value = false
  }
}

// Button "К текущей строке" clicked: re-center and resume auto-tracking
function resetToCurrentLine() {
  isUserScrolled.value = false
  scrollToActiveLine('smooth')
}

// Fetch lyrics from backend
async function fetchLyrics(force: boolean = false) {
  const tr = s.value.track || player.currentQueueTrack
  if (!tr) return

  const artist = displayArtist.value
  const title = displayTitle.value
  const album = displayAlbum.value
  const key = `${artist}:::${title}:::${tr.filePath || ''}`

  if (!force && key === lastLoadedKey.value && lyricsData.value) {
    return
  }
  // On force-retry also clear the key so state is fully reset
  if (force) {
    lastLoadedKey.value = ''
  }
  lastLoadedKey.value = key

  // Reset scroll and manual state on new track
  if (lyricsContainerRef.value) {
    lyricsContainerRef.value.scrollTop = 0
  }
  isUserScrolled.value = false

  isLoadingLyrics.value = true
  lyricsError.value = ''

  try {
    const res = await GetLyrics(
      artist,
      title,
      album,
      s.value.durationSec || 0,
      tr.filePath || '',
      force        // bypass backend cache when user clicked "Retry"
    )
    lyricsData.value = res as any
    nextTick(() => {
      scrollToActiveLine('auto')
    })
  } catch (err: any) {
    lyricsError.value = err?.message || 'Error loading lyrics'
    lyricsData.value = null
  } finally {
    isLoadingLyrics.value = false
  }
}

// Watch track change & fullscreen status
watch(
  [() => s.value.track?.filePath, () => player.currentQueueTrack?.filePath, () => player.isCoverFullscreen],
  ([newPath, newQPath, isFs]) => {
    if (isFs && (newPath || newQPath)) {
      window.scrollTo(0, 0)
      isUserScrolled.value = false
      fetchLyrics()
    }
  },
  { immediate: true }
)

function formatTime(sec: number): string {
  if (!sec || isNaN(sec)) return '0:00'
  const m = Math.floor(sec / 60)
  const secRemaining = Math.floor(sec % 60)
  return `${m}:${secRemaining.toString().padStart(2, '0')}`
}

const progress = computed(() => {
  if (!s.value.durationSec) return 0
  return Math.min(100, Math.max(0, (s.value.positionSec / s.value.durationSec) * 100))
})

function onSeekClick(e: MouseEvent) {
  if (!s.value.durationSec) return
  const container = e.currentTarget as HTMLElement
  const bar = container.querySelector('.fs-progress-bar') as HTMLElement
  if (!bar) return
  const rect = bar.getBoundingClientRect()
  let clickX = e.clientX - rect.left
  if (clickX < 0) clickX = 0
  if (clickX > rect.width) clickX = rect.width
  const ratio = clickX / rect.width
  player.seek(ratio * s.value.durationSec)
}

function seekToLine(time: number) {
  isUserScrolled.value = false
  player.seek(time)
  nextTick(() => {
    scrollToActiveLine('smooth')
  })
}

// Volume Controls
const volumePercent = computed(() => {
  const v = s.value.volume ?? 0.8
  return Math.min(100, Math.max(0, Math.round(v * 100)))
})

const volumeIcon = computed(() => {
  if (s.value.volume === 0) return '🔇'
  if (s.value.volume < 0.4) return '🔈'
  if (s.value.volume < 0.7) return '🔉'
  return '🔊'
})

let prevVolume = 0.8
function toggleMute() {
  if (s.value.volume > 0) {
    prevVolume = s.value.volume
    player.setVolume(0)
  } else {
    player.setVolume(prevVolume || 0.8)
  }
}

function handleKeyDown(e: KeyboardEvent) {
  if (!player.isCoverFullscreen) return
  if (e.key === 'Escape') {
    player.closeCoverFullscreen()
  } else if (e.code === 'Space' && (e.target as HTMLElement).tagName !== 'INPUT') {
    e.preventDefault()
    player.togglePause()
  } else if (e.key.toLowerCase() === 'l') {
    showLyrics.value = !showLyrics.value
  }
}

function onWindowResize() {
  if (!isUserScrolled.value) {
    scrollToActiveLine('auto')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
  window.addEventListener('resize', onWindowResize)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
  window.removeEventListener('resize', onWindowResize)
})
</script>

<template>
  <Transition name="fs-fade">
    <div v-if="player.isCoverFullscreen" class="fullscreen-cover-overlay">
      <!-- Ambient blurred background -->
      <div
        class="fs-ambient-bg"
        :style="coverUrl ? { backgroundImage: `url(${coverUrl})` } : {}"
      ></div>
      <div class="fs-backdrop-shade"></div>

      <!-- Top bar -->
      <header class="fs-top-bar">
        <div class="fs-brand">
          <span class="fs-brand-icon">🌊</span>
          <span class="fs-brand-text">{{ t('player.nowPlaying') }}</span>
        </div>

        <!-- Center / Right Top Actions -->
        <div class="fs-top-actions">
          <!-- Lyrics toggle button -->
          <button
            class="fs-lyrics-toggle-btn"
            :class="{ active: showLyrics }"
            @click="showLyrics = !showLyrics"
            :title="t('player.toggleLyrics') + ' (L)'"
          >
            <span class="fs-mic-icon">🎤</span>
            <span class="fs-lyrics-toggle-text">{{ t('player.lyrics') }}</span>
          </button>

          <!-- Close button -->
          <button
            class="fs-close-btn"
            @click="player.closeCoverFullscreen"
            :title="t('player.collapseCover')"
          >
            <span class="fs-close-icon">✕</span>
            <span class="fs-close-hint">Esc</span>
          </button>
        </div>
      </header>

      <!-- Center content: Cover on Left + Borderless Centered Lyrics on Right -->
      <main class="fs-main-content" :class="{ 'with-lyrics': showLyrics }">
        <!-- Cover Art & Metadata -->
        <div class="fs-visual-column">
          <div class="fs-cover-wrapper">
            <img
              v-if="coverUrl"
              :src="coverUrl"
              class="fs-cover-img"
              alt="Album Cover"
              @error="($event.target as HTMLElement).style.display = 'none'"
            />
            <div v-else class="fs-cover-placeholder">
              <span class="fs-placeholder-icon">🎵</span>
              <span class="fs-placeholder-text">{{ t('player.noCover') }}</span>
            </div>
          </div>

          <!-- Track Metadata -->
          <div class="fs-meta">
            <h1 class="fs-title" :title="displayTitle">{{ displayTitle }}</h1>
            <div class="fs-artist" :title="displayArtist">{{ displayArtist }}</div>
            <div class="fs-album" v-if="displayAlbum" :title="displayAlbum">{{ displayAlbum }}</div>

            <!-- Save Streamed Track Button -->
            <button
              v-if="s.track && s.state !== 'stopped'"
              class="fs-save-stream-btn"
              :class="{
                'is-saved': player.isCurrentSaved,
                'is-saving': player.isSavingTrack,
                'can-save': !player.isCurrentSaved
              }"
              :disabled="player.isCurrentSaved || player.isSavingTrack"
              @click="handleSaveTrack"
              :title="player.isCurrentSaved ? t('player.savedInLibrary') : (player.isSavingTrack ? t('player.savingTrack') : t('player.saveTrackBtn'))"
            >
              <span v-if="player.isSavingTrack" class="btn-spinner">⏳ {{ t('player.savingTrack') }}</span>
              <span v-else-if="player.isCurrentSaved" class="btn-check">✓ {{ t('player.savedInLibrary') }}</span>
              <span v-else class="btn-save-content">💾 {{ t('player.saveTrackBtn') }}</span>
            </button>
          </div>
        </div>

        <!-- Clean, Borderless Lyrics Stream (Current line always in center) -->
        <section v-if="showLyrics" class="fs-lyrics-column">
          <div
            ref="lyricsContainerRef"
            class="fs-lyrics-stream"
            @wheel.passive="onUserScrollInteraction"
            @touchmove.passive="onUserScrollInteraction"
            @pointerdown.passive="onUserScrollInteraction"
            @scroll.passive="onContainerScroll"
          >
            <!-- Loading State -->
            <div v-if="isLoadingLyrics" class="fs-lyrics-state fs-lyrics-loading">
              <div class="fs-soundwave">
                <span></span><span></span><span></span><span></span><span></span>
              </div>
              <div class="fs-state-text">{{ t('player.lyricsSearching') }}</div>
            </div>

            <!-- Instrumental State -->
            <div v-else-if="lyricsData?.instrumental" class="fs-lyrics-state fs-lyrics-instrumental">
              <div class="fs-instrumental-disc">🎷</div>
              <div class="fs-state-title">{{ t('player.lyricsInstrumental') }}</div>
              <div class="fs-state-sub">Наслаждайтесь чистым звучанием музыки</div>
            </div>

            <!-- Synced Lyrics Stream (Without Timestamps, Clean Text Only, Centered) -->
            <div v-else-if="syncedLines.length > 0" class="fs-synced-list">
              <div
                v-for="(line, idx) in syncedLines"
                :key="idx"
                :data-index="idx"
                class="fs-lyric-line"
                :class="{
                  'is-active': idx === activeLineIndex,
                  'is-past': idx < activeLineIndex,
                  'is-future': idx > activeLineIndex
                }"
                @click="seekToLine(line.time)"
              >
                <span class="fs-line-text">{{ line.text || '♪' }}</span>
              </div>
            </div>

            <!-- Plain Lyrics Fallback -->
            <div v-else-if="plainLines.length > 0" class="fs-plain-list">
              <div
                v-for="(line, idx) in plainLines"
                :key="idx"
                class="fs-plain-line"
              >
                {{ line }}
              </div>
            </div>

            <!-- Empty / Not Found State -->
            <div v-else class="fs-lyrics-state fs-lyrics-empty">
              <div class="fs-empty-icon">📜</div>
              <div class="fs-state-title">{{ t('player.lyricsNotFound') }}</div>
              <button class="fs-retry-btn" @click="fetchLyrics(true)">
                {{ t('player.retryLyrics') }}
              </button>
            </div>
          </div>

          <!-- Resume auto-sync button: appears ONLY when user manually scrolled away -->
          <Transition name="fade-quick">
            <button
              v-if="isUserScrolled && activeLineIndex >= 0 && syncedLines.length > 0"
              class="fs-resync-btn"
              @click="resetToCurrentLine"
            >
              <span>🎯</span>
              <span>{{ t('player.syncLyrics') }}</span>
            </button>
          </Transition>
        </section>
      </main>

      <!-- Bottom controls bar (with volume slider) -->
      <footer class="fs-controls-section">
        <!-- Progress bar -->
        <div class="fs-timeline-container" @click="onSeekClick">
          <span class="fs-time fs-time-curr">{{ formatTime(s.positionSec) }}</span>
          <div class="fs-progress-bar">
            <div class="fs-progress-fill" :style="{ width: progress + '%' }">
              <div class="fs-progress-handle"></div>
            </div>
          </div>
          <span class="fs-time fs-time-total">{{ formatTime(s.durationSec) }}</span>
        </div>

        <!-- Controls Row -->
        <div class="fs-controls-row">
          <!-- Left spacer for balance -->
          <div class="fs-controls-side fs-side-left"></div>

          <!-- Transport buttons (Center) -->
          <div class="fs-buttons-row">
            <!-- Shuffle -->
            <button
              class="fs-btn fs-shuffle-btn"
              :class="{ active: player.isShuffled }"
              @click="player.toggleShuffle"
              :title="player.isShuffled ? t('player.shuffleOn') : t('player.shuffleOff')"
            >
              🔀
            </button>

            <!-- Prev -->
            <button
              class="fs-btn fs-prev-btn"
              @click="player.playPrev"
              :disabled="player.playlist.length === 0"
              :title="t('player.prevTrack')"
            >
              <svg viewBox="0 0 24 24" class="fs-nav-svg">
                <path fill="currentColor" d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/>
              </svg>
            </button>

            <!-- Play / Pause -->
            <button
              class="fs-play-btn"
              @click="player.togglePause"
              :disabled="player.playlist.length === 0 && !s.track"
              :title="s.state === 'playing' ? t('common.pause') : t('common.play')"
            >
              <svg v-if="s.state === 'playing'" viewBox="0 0 24 24" class="fs-play-svg">
                <path fill="currentColor" d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
              </svg>
              <svg v-else viewBox="0 0 24 24" class="fs-play-svg fs-triangle">
                <path fill="currentColor" d="M8 5v14l11-7z"/>
              </svg>
            </button>

            <!-- Next -->
            <button
              class="fs-btn fs-next-btn"
              @click="player.playNext"
              :disabled="player.playlist.length === 0"
              :title="t('player.nextTrack')"
            >
              <svg viewBox="0 0 24 24" class="fs-nav-svg">
                <path fill="currentColor" d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/>
              </svg>
            </button>

            <!-- Repeat -->
            <button
              class="fs-btn fs-repeat-btn"
              :class="{ active: player.repeatMode !== 'off' }"
              @click="player.toggleRepeat"
              :title="player.repeatMode === 'one' ? t('player.repeatOne') : (player.repeatMode === 'all' ? t('player.repeatAll') : t('player.repeatOff'))"
            >
              <span v-if="player.repeatMode === 'one'">🔂</span>
              <span v-else>🔁</span>
            </button>
          </div>

          <!-- Volume Control (Right) -->
          <div class="fs-controls-side fs-side-right">
            <div class="fs-volume-wrap">
              <span
                class="fs-volume-icon"
                @click="toggleMute"
                :title="s.volume === 0 ? t('player.unmute') : t('player.mute')"
              >
                {{ volumeIcon }}
              </span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                :value="s.volume"
                @input="player.setVolume(+($event.target as HTMLInputElement).value)"
                class="fs-volume-slider"
                :style="{
                  background: `linear-gradient(to right, var(--accent) 0%, var(--accent) ${volumePercent}%, rgba(255, 255, 255, 0.2) ${volumePercent}%, rgba(255, 255, 255, 0.2) 100%)`
                }"
                :title="t('player.volume')"
              />
            </div>
          </div>
        </div>
      </footer>
    </div>
  </Transition>
</template>

<style scoped>
.fullscreen-cover-overlay {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  z-index: 99999;
  background: #0d0d0d;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  overscroll-behavior: none;
  user-select: none;
}

/* Ambient glow background */
.fs-ambient-bg {
  position: absolute;
  inset: -60px;
  background-size: cover;
  background-position: center;
  filter: blur(90px) saturate(2.2) brightness(0.24);
  transform: scale(1.15);
  z-index: 1;
  pointer-events: none;
}

.fs-backdrop-shade {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at center, rgba(16, 16, 16, 0.45) 0%, rgba(8, 8, 8, 0.94) 100%);
  z-index: 2;
  pointer-events: none;
}

/* Header */
.fs-top-bar {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 36px 0;
  flex-shrink: 0;
}

.fs-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text2);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.fs-brand-icon {
  font-size: 18px;
}

.fs-top-actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.fs-lyrics-toggle-btn {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 16px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 20px;
  color: var(--text2);
  font-size: 13px;
  font-weight: 600;
  transition: all 0.2s ease;
  backdrop-filter: blur(12px);
  cursor: pointer;
}

.fs-lyrics-toggle-btn.active {
  background: rgba(29, 185, 84, 0.22);
  border-color: rgba(29, 185, 84, 0.5);
  color: #fff;
  box-shadow: 0 0 16px rgba(29, 185, 84, 0.25);
}

.fs-close-btn {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: var(--text);
  border-radius: 24px;
  padding: 7px 15px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  transition: all 0.15s ease;
  backdrop-filter: blur(10px);
}

.fs-close-btn:hover {
  background: rgba(255, 255, 255, 0.18);
  border-color: rgba(255, 255, 255, 0.3);
  transform: scale(1.03);
}

.fs-close-icon {
  font-size: 14px;
  line-height: 1;
}

.fs-close-hint {
  font-size: 11px;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text2);
}

/* Main Content Area */
.fs-main-content {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 1360px;
  margin: 0 auto;
  padding: 10px 48px;
  gap: 60px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.fs-visual-column {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  flex: 1;
  max-width: 480px;
  min-width: 0;
  transition: all 0.3s ease;
}

.fs-main-content:not(.with-lyrics) .fs-visual-column {
  max-width: 540px;
}

/* Album Cover */
.fs-cover-wrapper {
  width: 100%;
  max-width: min(46vh, 420px);
  aspect-ratio: 1 / 1;
  border-radius: 20px;
  overflow: hidden;
  background: #181818;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.8), 0 0 0 1px rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.3s ease;
}

.fs-cover-wrapper:hover {
  transform: scale(1.015);
}

.fs-cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.fs-cover-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  color: var(--text2);
}

.fs-placeholder-icon {
  font-size: 80px;
}

.fs-placeholder-text {
  font-size: 15px;
  opacity: 0.6;
}

/* Track Metadata */
.fs-meta {
  text-align: center;
  margin-top: 24px;
  width: 100%;
}

.fs-title {
  font-size: clamp(20px, 3vh, 28px);
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.3px;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.6);
}

.fs-artist {
  font-size: clamp(14px, 2vh, 18px);
  font-weight: 500;
  color: var(--accent);
  margin-top: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.6);
}

.fs-album {
  font-size: 13px;
  color: var(--text2);
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Lyrics Column (Right Side) — Completely Borderless & Clean */
.fs-lyrics-column {
  position: relative;
  flex: 1.3;
  height: 100%;
  max-height: calc(100vh - 210px);
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  animation: fs-slide-in 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes fs-slide-in {
  from {
    opacity: 0;
    transform: translateX(20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

/* Lyrics Scrollable Stream */
.fs-lyrics-stream {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  /* Top and bottom padding allow first and last lines to be positioned directly in the vertical center */
  padding-top: calc(50vh - 80px);
  padding-bottom: calc(50vh - 80px);
  padding-left: 12px;
  padding-right: 12px;
  scroll-behavior: smooth;
  position: relative;
  mask-image: linear-gradient(to bottom, transparent 0%, black 20%, black 80%, transparent 100%);
  -webkit-mask-image: linear-gradient(to bottom, transparent 0%, black 20%, black 80%, transparent 100%);
}

.fs-lyrics-stream::-webkit-scrollbar {
  display: none;
}
.fs-lyrics-stream {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

/* Synced Line (Running Karaoke Highlight, Clean Text Only) */
.fs-synced-list {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.fs-lyric-line {
  padding: 8px 14px;
  border-radius: 12px;
  cursor: pointer;
  transition: color 0.3s ease, opacity 0.3s ease, transform 0.3s ease, text-shadow 0.3s ease;
  line-height: 1.5;
  user-select: text;
}

.fs-lyric-line.is-past {
  opacity: 0.26;
  color: var(--text);
  filter: blur(0.2px);
  transform: scale(0.96);
  transform-origin: left center;
}

.fs-lyric-line.is-future {
  opacity: 0.55;
  color: var(--text);
  transform: scale(0.98);
  transform-origin: left center;
}

.fs-lyric-line.is-active {
  opacity: 1;
  color: #ffffff;
  font-weight: 700;
  transform: scale(1.08);
  transform-origin: left center;
  text-shadow: 0 0 24px rgba(255, 255, 255, 0.5), 0 0 45px rgba(29, 185, 84, 0.45);
}

.fs-lyric-line:hover {
  opacity: 0.95;
}

.fs-line-text {
  font-size: clamp(20px, 2.8vh, 30px);
  word-break: break-word;
  transition: color 0.25s ease;
}

/* Plain lyrics fallback */
.fs-plain-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  color: var(--text);
  opacity: 0.8;
  line-height: 1.6;
}

.fs-plain-line {
  font-size: clamp(18px, 2.5vh, 26px);
  padding: 4px 8px;
}

/* Empty, Loading & Instrumental States */
.fs-lyrics-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 40px 20px;
  height: 100%;
}

.fs-soundwave {
  display: flex;
  align-items: center;
  gap: 5px;
  height: 40px;
  margin-bottom: 16px;
}

.fs-soundwave span {
  width: 4px;
  background: var(--accent);
  border-radius: 2px;
  animation: soundwave 1.2s ease-in-out infinite;
}

.fs-soundwave span:nth-child(1) { height: 16px; animation-delay: 0.0s; }
.fs-soundwave span:nth-child(2) { height: 32px; animation-delay: 0.2s; }
.fs-soundwave span:nth-child(3) { height: 24px; animation-delay: 0.4s; }
.fs-soundwave span:nth-child(4) { height: 38px; animation-delay: 0.1s; }
.fs-soundwave span:nth-child(5) { height: 20px; animation-delay: 0.3s; }

@keyframes soundwave {
  0%, 100% { transform: scaleY(0.4); }
  50% { transform: scaleY(1); }
}

.fs-state-title {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
  margin-top: 10px;
}

.fs-state-sub {
  font-size: 13px;
  color: var(--text2);
  margin-top: 6px;
  max-width: 320px;
}

.fs-instrumental-disc {
  font-size: 50px;
}

.fs-empty-icon {
  font-size: 44px;
  opacity: 0.6;
}

.fs-retry-btn {
  margin-top: 18px;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  padding: 8px 18px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.fs-retry-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

/* Floating Re-sync button */
.fs-resync-btn {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--accent);
  color: #000;
  border: none;
  padding: 9px 20px;
  border-radius: 26px;
  font-size: 13px;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 8px;
  box-shadow: 0 8px 24px rgba(29, 185, 84, 0.55), 0 2px 8px rgba(0, 0, 0, 0.4);
  cursor: pointer;
  z-index: 25;
  transition: transform 0.15s ease, background 0.15s ease;
}

.fs-resync-btn:hover {
  background: #22d460;
  transform: translateX(-50%) scale(1.06);
}

/* Controls Section (Bottom) */
.fs-controls-section {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 880px;
  margin: 0 auto;
  padding: 0 36px 28px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  flex-shrink: 0;
}

/* Timeline */
.fs-timeline-container {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  cursor: pointer;
  padding: 6px 0;
}

.fs-time {
  font-size: 13px;
  color: var(--text2);
  font-variant-numeric: tabular-nums;
  min-width: 42px;
}

.fs-time-curr { text-align: right; }
.fs-time-total { text-align: left; }

.fs-progress-bar {
  flex: 1;
  height: 6px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 3px;
  position: relative;
  overflow: visible;
  transition: height 0.15s ease;
}

.fs-timeline-container:hover .fs-progress-bar {
  height: 8px;
}

.fs-progress-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 4px;
  position: relative;
  transition: width 0.1s linear;
}

.fs-progress-handle {
  width: 14px;
  height: 14px;
  background: #fff;
  border-radius: 50%;
  position: absolute;
  right: -7px;
  top: 50%;
  transform: translateY(-50%) scale(0);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
  transition: transform 0.15s ease;
}

.fs-timeline-container:hover .fs-progress-handle {
  transform: translateY(-50%) scale(1);
}

/* Controls Row with Volume */
.fs-controls-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.fs-controls-side {
  flex: 1;
  display: flex;
  align-items: center;
}

.fs-side-left {
  justify-content: flex-start;
}

.fs-side-right {
  justify-content: flex-end;
}

/* Volume Slider */
.fs-volume-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.fs-volume-icon {
  font-size: 18px;
  cursor: pointer;
  user-select: none;
  opacity: 0.8;
  transition: transform 0.1s, opacity 0.15s;
}

.fs-volume-icon:hover {
  opacity: 1;
  transform: scale(1.1);
}

.fs-volume-slider {
  -webkit-appearance: none;
  appearance: none;
  width: 100px;
  height: 4px;
  border-radius: 2px;
  outline: none;
  cursor: pointer;
  transition: height 0.12s ease;
}

.fs-volume-slider:hover {
  height: 6px;
}

.fs-volume-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.6);
  transition: transform 0.1s;
}

.fs-volume-slider:hover::-webkit-slider-thumb {
  transform: scale(1.2);
}

/* Buttons Row */
.fs-buttons-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 22px;
}

.fs-btn {
  width: 44px;
  height: 44px;
  min-width: 44px;
  min-height: 44px;
  aspect-ratio: 1 / 1;
  background: transparent;
  border: none;
  color: var(--text);
  font-size: 20px;
  cursor: pointer;
  padding: 0;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  flex-shrink: 0;
  opacity: 0.75;
  transition: all 0.15s ease;
}

.fs-btn:hover:not(:disabled) {
  opacity: 1;
  background: rgba(255, 255, 255, 0.1);
  transform: scale(1.1);
}

.fs-btn:disabled {
  opacity: 0.25;
  cursor: not-allowed;
}

.fs-nav-svg {
  width: 24px;
  height: 24px;
  display: block;
}

.fs-shuffle-btn.active,
.fs-repeat-btn.active {
  color: var(--accent);
  opacity: 1;
  background: rgba(29, 185, 84, 0.18);
}

.fs-play-btn {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: var(--accent);
  color: #000;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 24px rgba(29, 185, 84, 0.4);
  transition: all 0.15s ease;
}

.fs-play-btn:hover:not(:disabled) {
  background: #22d460;
  transform: scale(1.08);
  box-shadow: 0 10px 30px rgba(29, 185, 84, 0.6);
}

.fs-play-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.fs-play-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.fs-play-svg {
  width: 26px;
  height: 26px;
  display: block;
}

.fs-play-svg.fs-triangle {
  transform: translateX(2px);
}

/* Transitions */
.fs-fade-enter-active,
.fs-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.fs-fade-enter-from,
.fs-fade-leave-to {
  opacity: 0;
  transform: scale(0.97);
}

.fade-quick-enter-active,
.fade-quick-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-quick-enter-from,
.fade-quick-leave-to {
  opacity: 0;
  transform: translate(-50%, 10px);
}

/* Responsive breakpoint */
@media (max-width: 960px) {
  .fs-main-content {
    flex-direction: column;
    padding: 10px 20px;
    gap: 20px;
  }

  .fs-lyrics-column {
    width: 100%;
    max-height: 40vh;
  }

  .fs-cover-wrapper {
    max-width: 200px;
  }

  .fs-volume-wrap {
    display: none;
  }
}

/* Save Streamed Track Button in Fullscreen */
.fs-save-stream-btn {
  margin-top: 14px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: #ffffff;
  border-radius: 20px;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  backdrop-filter: blur(10px);
}

.fs-save-stream-btn.can-save:hover {
  background: rgba(29, 185, 84, 0.22);
  border-color: var(--accent);
  color: #ffffff;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(29, 185, 84, 0.25);
}

.fs-save-stream-btn.is-saved {
  background: rgba(29, 185, 84, 0.15);
  border-color: rgba(29, 185, 84, 0.4);
  color: var(--accent);
  cursor: default;
}

.fs-save-stream-btn.is-saving {
  cursor: wait;
}

.btn-save-content {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
</style>
