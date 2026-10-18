<script lang="ts" setup>
import { computed, onMounted, onUnmounted } from 'vue'
import { usePlayerStore } from '../store/player'
import { useI18n } from '../i18n'

const player = usePlayerStore()
const { t } = useI18n()
const s = computed(() => player.status)

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
  if (s.value.state === 'stopped' || !s.value.durationSec) return
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

function handleKeyDown(e: KeyboardEvent) {
  if (!player.isCoverFullscreen) return
  if (e.key === 'Escape') {
    player.closeCoverFullscreen()
  } else if (e.code === 'Space' && (e.target as HTMLElement).tagName !== 'INPUT') {
    e.preventDefault()
    player.togglePause()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
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

        <button
          class="fs-close-btn"
          @click="player.closeCoverFullscreen"
          :title="t('player.collapseCover')"
        >
          <span class="fs-close-icon">✕</span>
          <span class="fs-close-hint">Esc</span>
        </button>
      </header>

      <!-- Center content: Cover & track info -->
      <main class="fs-main">
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

        <div class="fs-meta">
          <h1 class="fs-title" :title="displayTitle">{{ displayTitle }}</h1>
          <div class="fs-artist" :title="displayArtist">{{ displayArtist }}</div>
          <div class="fs-album" v-if="displayAlbum" :title="displayAlbum">{{ displayAlbum }}</div>
        </div>
      </main>

      <!-- Bottom controls bar -->
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

        <!-- Transport buttons -->
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
            title="Предыдущий трек"
          >
            ⏮
          </button>

          <!-- Play / Pause -->
          <button
            class="fs-play-btn"
            @click="player.togglePause"
            :disabled="s.state === 'stopped' && !player.currentQueueTrack"
            :title="s.state === 'playing' ? t('common.pause') : t('common.play')"
          >
            <!-- Pause Icon -->
            <svg v-if="s.state === 'playing'" viewBox="0 0 24 24" class="fs-play-svg">
              <path fill="currentColor" d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
            </svg>
            <!-- Play Icon -->
            <svg v-else viewBox="0 0 24 24" class="fs-play-svg fs-triangle">
              <path fill="currentColor" d="M8 5v14l11-7z"/>
            </svg>
          </button>

          <!-- Next -->
          <button
            class="fs-btn fs-next-btn"
            @click="player.playNext"
            :disabled="player.playlist.length === 0"
            title="Следующий трек"
          >
            ⏭
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
      </footer>
    </div>
  </Transition>
</template>

<style scoped>
.fullscreen-cover-overlay {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: #0d0d0d;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  user-select: none;
}

/* Ambient glow background */
.fs-ambient-bg {
  position: absolute;
  inset: -60px;
  background-size: cover;
  background-position: center;
  filter: blur(80px) saturate(2) brightness(0.25);
  transform: scale(1.15);
  z-index: 1;
  pointer-events: none;
}

.fs-backdrop-shade {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at center, rgba(18, 18, 18, 0.4) 0%, rgba(10, 10, 10, 0.9) 100%);
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
  padding: 24px 36px 0;
}

.fs-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text2);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.fs-brand-icon {
  font-size: 20px;
}

.fs-close-btn {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.15);
  color: var(--text);
  border-radius: 24px;
  padding: 8px 16px;
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
  font-size: 15px;
  line-height: 1;
}

.fs-close-hint {
  font-size: 11px;
  background: rgba(255, 255, 255, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text2);
}

/* Main Center Content */
.fs-main {
  position: relative;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 10px 30px;
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
}

.fs-cover-wrapper {
  width: 100%;
  max-width: min(52vh, 460px);
  aspect-ratio: 1 / 1;
  border-radius: 20px;
  overflow: hidden;
  background: #1c1c1c;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.75), 0 0 0 1px rgba(255, 255, 255, 0.08);
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
  font-size: 90px;
}

.fs-placeholder-text {
  font-size: 16px;
  font-weight: 500;
  opacity: 0.6;
}

.fs-meta {
  text-align: center;
  margin-top: 28px;
  width: 100%;
}

.fs-title {
  font-size: clamp(20px, 3.2vh, 28px);
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
  font-size: 14px;
  color: var(--text2);
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Controls Section */
.fs-controls-section {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 680px;
  margin: 0 auto;
  padding: 0 30px 36px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}

/* Timeline */
.fs-timeline-container {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  cursor: pointer;
  padding: 8px 0;
}

.fs-time {
  font-size: 13px;
  color: var(--text2);
  font-variant-numeric: tabular-nums;
  min-width: 42px;
}

.fs-time-curr {
  text-align: right;
}

.fs-time-total {
  text-align: left;
}

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

/* Buttons Row */
.fs-buttons-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 22px;
}

.fs-btn {
  background: transparent;
  border: none;
  color: var(--text);
  font-size: 22px;
  cursor: pointer;
  padding: 8px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
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

/* Animations */
.fs-fade-enter-active,
.fs-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.fs-fade-enter-from,
.fs-fade-leave-to {
  opacity: 0;
  transform: scale(0.97);
}
</style>
