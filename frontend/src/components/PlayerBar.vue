<script lang="ts" setup>
import { usePlayerStore } from '../store/player'
import { useI18n } from '../i18n'
import { computed } from 'vue'

const player = usePlayerStore()
const { t } = useI18n()
const s = computed(() => player.status)

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
  const bar = container.querySelector('.progress-bar') as HTMLElement
  if (!bar) return
  const rect = bar.getBoundingClientRect()
  let clickX = e.clientX - rect.left
  if (clickX < 0) clickX = 0
  if (clickX > rect.width) clickX = rect.width
  const ratio = clickX / rect.width
  player.seek(ratio * s.value.durationSec)
}

const repeatTooltip = computed(() => {
  if (player.repeatMode === 'one') return t('player.repeatOne')
  if (player.repeatMode === 'all') return t('player.repeatAll')
  return t('player.repeatOff')
})

const volumeIcon = computed(() => {
  if (s.value.volume === 0) return '🔇'
  if (s.value.volume < 0.4) return '🔈'
  if (s.value.volume < 0.7) return '🔉'
  return '🔊'
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
  const a = s.value.track?.artist || player.currentQueueTrack?.artist
  if (a && a !== 'Неизвестный исполнитель') return a
  if (a === 'Неизвестный исполнитель') return t('library.unknownArtist')
  const fn = s.value.track?.fileName || player.currentQueueTrack?.fileName || ''
  if (!fn) return ''
  const nameWithoutExt = fn.replace(/\.[^/.]+$/, '')
  if (nameWithoutExt.includes(' - ')) {
    return nameWithoutExt.split(' - ')[0].trim()
  }
  return ''
})

const volumePercent = computed(() => {
  const v = s.value.volume ?? 0.8
  return Math.min(100, Math.max(0, Math.round(v * 100)))
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
</script>

<template>
  <footer class="player-bar">
    <!-- Track info -->
    <div class="track-info">
      <div
        class="track-cover-box"
        @click="player.openCoverFullscreen()"
        :title="t('player.expandCover')"
        role="button"
        tabindex="0"
      >
        <img
          v-if="s.track?.coverPath"
          :src="'/api/cover?path=' + encodeURIComponent(s.track.coverPath)"
          class="track-cover-img"
          alt="Cover"
          @error="($event.target as HTMLElement).style.display = 'none'"
        />
        <div v-else class="track-icon">🎵</div>

        <div class="track-cover-overlay">
          <span class="cover-expand-icon">⛶</span>
        </div>
      </div>
      <div class="track-meta">
        <div class="track-name" :title="displayTitle">{{ displayTitle }}</div>
        <div class="track-artist" v-if="displayArtist" :title="displayArtist">{{ displayArtist }}</div>
        <div class="track-sub" v-else>
          <span v-if="s.state === 'playing'" class="state-indicator playing">▶ {{ t('common.play') }}</span>
          <span v-else-if="s.state === 'paused'" class="state-indicator paused">⏸ {{ t('common.pause') }}</span>
          <span v-else class="state-indicator stopped">{{ t('player.noTrack') }}</span>
        </div>
      </div>
    </div>

    <!-- Controls + progress -->
    <div class="controls-wrap">
      <div class="controls">
        <!-- Shuffle Button (left of controls) -->
        <button
          class="ctrl-btn shuffle-btn"
          :class="{ active: player.isShuffled }"
          @click="player.toggleShuffle()"
          :title="player.isShuffled ? t('player.shuffleOn') : t('player.shuffleOff')"
        >
          🔀
        </button>

        <button
          class="ctrl-btn"
          @click="player.playPrev()"
          :disabled="s.state === 'stopped'"
          :title="t('player.prevTrack')"
        >
          ⏮
        </button>

        <!-- Perfectly Centered Play/Pause Button -->
        <button
          class="ctrl-btn play-btn"
          @click="player.togglePause()"
          :disabled="s.state === 'stopped'"
          :title="s.state === 'playing' ? t('common.pause') : t('common.play')"
        >
          <svg v-if="s.state === 'playing'" viewBox="0 0 24 24" class="play-svg">
            <path fill="currentColor" d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
          </svg>
          <svg v-else viewBox="0 0 24 24" class="play-svg play-triangle">
            <path fill="currentColor" d="M8 5v14l11-7z"/>
          </svg>
        </button>

        <button
          class="ctrl-btn"
          @click="player.playNext()"
          :disabled="s.state === 'stopped'"
          :title="t('player.nextTrack')"
        >
          ⏭
        </button>

        <!-- Repeat Button (right of controls) -->
        <button
          class="ctrl-btn repeat-btn"
          :class="player.repeatMode"
          @click="player.toggleRepeat()"
          :title="repeatTooltip"
        >
          {{ player.repeatMode === 'one' ? '🔂' : '🔁' }}
        </button>
      </div>

      <!-- Enhanced Progress Bar with Hover Expansion -->
      <div
        class="progress-container"
        @click="onSeekClick"
        :class="{ inactive: s.state === 'stopped' }"
      >
        <span class="time-label">{{ formatTime(s.positionSec) }}</span>
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: progress + '%' }">
            <div class="progress-thumb"></div>
          </div>
        </div>
        <span class="time-label">{{ formatTime(s.durationSec) }}</span>
      </div>
    </div>

    <!-- Volume & Fullscreen Button -->
    <div class="right-actions-wrap">
      <div class="volume-wrap">
        <span class="volume-icon" @click="toggleMute" :title="s.volume === 0 ? t('player.unmute') : t('player.mute')">{{ volumeIcon }}</span>
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          :value="s.volume"
          @input="player.setVolume(+($event.target as HTMLInputElement).value)"
          class="volume-slider"
          :style="{
            background: `linear-gradient(to right, var(--accent) 0%, var(--accent) ${volumePercent}%, var(--surface2) ${volumePercent}%, var(--surface2) 100%)`
          }"
          :title="t('player.volume')"
        />
      </div>

      <button
        class="fs-toggle-btn lyrics-btn"
        @click="player.openCoverFullscreen()"
        :title="t('player.lyrics')"
      >
        <span>🎤</span>
      </button>

      <button
        class="fs-toggle-btn"
        @click="player.openCoverFullscreen()"
        :title="t('player.expandCover')"
      >
        <span>⛶</span>
      </button>
    </div>
  </footer>
</template>

<style scoped>
.player-bar {
  grid-column: 1 / 3;
  grid-row: 2;
  background: var(--bg2);
  border-top: 1px solid var(--border);
  display: grid;
  grid-template-columns: 1fr 2.4fr 1fr;
  align-items: center;
  padding: 0 24px;
  gap: 20px;
  height: var(--player-h);
}

/* Track info */
.track-info { display: flex; align-items: center; gap: 12px; min-width: 0; }
.track-cover-box {
  width: 44px;
  height: 44px;
  border-radius: 6px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  flex-shrink: 0;
  position: relative;
  cursor: pointer;
  transition: transform 0.15s ease;
}
.track-cover-box:hover {
  transform: scale(1.05);
}
.track-cover-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s ease;
  border-radius: 6px;
}
.track-cover-box:hover .track-cover-overlay {
  opacity: 1;
}
.cover-expand-icon {
  color: #fff;
  font-size: 16px;
  transform: scale(0.9);
  transition: transform 0.15s ease;
}
.track-cover-box:hover .cover-expand-icon {
  transform: scale(1.1);
}
.track-cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.track-icon { font-size: 26px; }
.track-meta { min-width: 0; }
.track-name { font-size: 14px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.track-artist { font-size: 12px; color: var(--text2); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 500; }
.track-sub  { font-size: 12px; color: var(--text2); margin-top: 2px; }
.state-indicator.playing { color: var(--accent); }
.state-indicator.paused { color: #ff9800; }
.state-indicator.stopped { opacity: 0.5; }

/* Controls */
.controls-wrap { display: flex; flex-direction: column; align-items: center; gap: 4px; width: 100%; max-width: 600px; margin: 0 auto; }
.controls { display: flex; align-items: center; gap: 10px; }
.ctrl-btn { font-size: 18px; opacity: 0.85; padding: 4px 8px; border-radius: 6px; transition: opacity 0.1s, background 0.1s; border: none; background: transparent; cursor: pointer; color: var(--text); }
.ctrl-btn:hover:not(:disabled) { opacity: 1; background: var(--surface); }
.ctrl-btn:disabled { opacity: 0.3; cursor: default; }

.play-btn {
  background: var(--accent); color: #000;
  border-radius: 50%; width: 40px; height: 40px;
  display: flex; align-items: center; justify-content: center;
  padding: 0;
  border: none;
  cursor: pointer;
  transition: transform 0.1s ease, background 0.15s ease;
}
.play-btn:hover:not(:disabled) {
  background: #22d460;
  transform: scale(1.05);
}
.play-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.play-svg {
  width: 18px;
  height: 18px;
  display: block;
}
/* Visual optical centering for the right-pointing triangle */
.play-svg.play-triangle {
  transform: translateX(1.5px);
}

.shuffle-btn {
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  padding: 4px 8px;
  opacity: 0.35;
  transition: all 0.15s ease;
}
.shuffle-btn.active {
  opacity: 1;
  color: var(--accent);
  background: rgba(29, 185, 84, 0.18);
}

.repeat-btn {
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  padding: 4px 8px;
  transition: all 0.15s ease;
}
.repeat-btn.off {
  opacity: 0.35;
}
.repeat-btn.all {
  opacity: 1;
  color: var(--accent);
  background: rgba(29, 185, 84, 0.15);
}
.repeat-btn.one {
  opacity: 1;
  color: #22d460;
  background: rgba(29, 185, 84, 0.25);
}

/* Progress Container and Bar */
.progress-container {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  cursor: pointer;
  user-select: none;
}
.progress-container.inactive {
  cursor: default;
  opacity: 0.4;
}

.time-label {
  font-size: 11px;
  color: var(--text2);
  min-width: 32px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.progress-bar {
  flex: 1;
  height: 5px;
  background: var(--surface2);
  border-radius: 4px;
  position: relative;
  transition: height 0.15s ease;
}

.progress-container:hover:not(.inactive) .progress-bar {
  height: 9px;
}

.progress-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 4px;
  position: relative;
  transition: background-color 0.15s ease;
}

.progress-container:hover:not(.inactive) .progress-fill {
  background: #22d460;
}

.progress-thumb {
  position: absolute;
  right: -6px;
  top: 50%;
  transform: translateY(-50%) scale(0);
  width: 14px;
  height: 14px;
  background: #ffffff;
  border-radius: 50%;
  box-shadow: 0 1px 4px rgba(0,0,0,0.5);
  transition: transform 0.12s ease;
  pointer-events: none;
}

.progress-container:hover:not(.inactive) .progress-thumb {
  transform: translateY(-50%) scale(1);
}

/* Volume & Actions */
.right-actions-wrap {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 14px;
}
.fs-toggle-btn {
  background: transparent;
  border: none;
  color: var(--text2);
  font-size: 19px;
  cursor: pointer;
  padding: 6px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  opacity: 0.8;
}
.fs-toggle-btn:hover {
  color: var(--accent);
  background: var(--surface);
  opacity: 1;
  transform: scale(1.1);
}
.volume-wrap { display: flex; align-items: center; gap: 10px; justify-content: flex-end; }
.volume-icon { font-size: 18px; user-select: none; cursor: pointer; transition: transform 0.1s; }
.volume-icon:hover { transform: scale(1.1); }
.volume-slider {
  -webkit-appearance: none;
  appearance: none;
  width: 96px;
  height: 4px;
  border-radius: 2px;
  outline: none;
  cursor: pointer;
  transition: height 0.12s ease;
}
.volume-slider:hover {
  height: 6px;
}
.volume-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 12px;
  height: 12px;
  background: #ffffff;
  border-radius: 50%;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.45);
  transition: transform 0.1s ease;
}
.volume-slider:hover::-webkit-slider-thumb {
  transform: scale(1.15);
}
.volume-slider::-moz-range-thumb {
  width: 12px;
  height: 12px;
  background: #ffffff;
  border: none;
  border-radius: 50%;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.45);
  transition: transform 0.1s ease;
}
.volume-slider:hover::-moz-range-thumb {
  transform: scale(1.15);
}
</style>
