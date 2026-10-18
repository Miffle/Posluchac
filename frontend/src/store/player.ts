import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { GetStatus, Pause, Stop, SeekTo, SetVolume, PlayTrack } from '../../wailsjs/go/main/App'

export interface PlayerStatus {
  state: 'playing' | 'paused' | 'stopped'
  track?: {
    infoHash: string
    filePath: string
    fileName: string
    title?: string
    artist?: string
    album?: string
    coverPath?: string
  }
  positionSec: number
  durationSec: number
  volume: number
}

export type RepeatMode = 'off' | 'all' | 'one'

export interface QueueTrack {
  infoHash: string
  filePath: string
  fileName: string
  title?: string
  artist?: string
  album?: string
  coverPath?: string
}

export const usePlayerStore = defineStore('player', () => {
  const savedVol = localStorage.getItem('torrent_player_volume')
  const initialVol = savedVol !== null ? parseFloat(savedVol) : 0.8
  const validVol = isNaN(initialVol) ? 0.8 : Math.max(0, Math.min(1, initialVol))

  const status = ref<PlayerStatus>({
    state: 'stopped',
    positionSec: 0,
    durationSec: 0,
    volume: validVol,
  })

  const savedShuffle = localStorage.getItem('torrent_player_shuffle') === 'true'
  const isShuffled = ref<boolean>(savedShuffle)
  const repeatMode = ref<RepeatMode>('off')
  const playlist = ref<QueueTrack[]>([])
  const currentIndex = ref<number>(-1)
  const isCoverFullscreen = ref<boolean>(false)

  // Shuffle queue & history for non-repeating shuffle cycle with backward navigation
  const unplayedIndices = ref<number[]>([])
  const shuffleHistory = ref<number[]>([])

  function initShufflePool(excludeIndex: number = -1) {
    const total = playlist.value.length
    if (total <= 1) {
      unplayedIndices.value = []
      return
    }
    const pool: number[] = []
    for (let i = 0; i < total; i++) {
      if (i !== excludeIndex) {
        pool.push(i)
      }
    }
    // Fisher-Yates shuffle
    for (let i = pool.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [pool[i], pool[j]] = [pool[j], pool[i]]
    }
    unplayedIndices.value = pool
  }

  function openCoverFullscreen() {
    isCoverFullscreen.value = true
  }

  function closeCoverFullscreen() {
    isCoverFullscreen.value = false
  }

  function toggleCoverFullscreen() {
    isCoverFullscreen.value = !isCoverFullscreen.value
  }

  let pollInterval: ReturnType<typeof setInterval> | null = null
  let lastState = 'stopped'
  let volumeInitialized = false

  function startPolling() {
    pollInterval = setInterval(async () => {
      try {
        if (!volumeInitialized) {
          await SetVolume(status.value.volume)
          volumeInitialized = true
        }

        const s = (await GetStatus()) as PlayerStatus
        const prev = lastState
        lastState = s.state
        status.value = {
          ...s,
          volume: status.value.volume
        }

        // Track finished naturally (was playing, now stopped)
        if (prev === 'playing' && s.state === 'stopped') {
          handleTrackEnded()
        }
      } catch { /* ignore */ }
    }, 500)
  }

  async function handleTrackEnded() {
    if (repeatMode.value === 'one' && status.value.track) {
      // Repeat same song
      await PlayTrack(status.value.track.infoHash, status.value.track.filePath)
    } else if (repeatMode.value === 'all' || isShuffled.value) {
      // Repeat playlist or shuffle: play next
      await playNext()
    } else if (currentIndex.value < playlist.value.length - 1) {
      // Play next song until end of list
      await playNext()
    }
  }

  function stopPolling() {
    if (pollInterval) clearInterval(pollInterval)
  }

  async function play(infoHash: string, filePath: string, queue?: QueueTrack[]) {
    if (queue && queue.length > 0) {
      playlist.value = queue
      currentIndex.value = queue.findIndex(t => t.filePath === filePath)
      shuffleHistory.value = []
      if (isShuffled.value) {
        initShufflePool(currentIndex.value)
      }
    } else {
      const idx = playlist.value.findIndex(t => t.filePath === filePath)
      if (idx !== -1) {
        currentIndex.value = idx
      } else {
        playlist.value.push({ infoHash, filePath, fileName: filePath })
        currentIndex.value = playlist.value.length - 1
      }
      shuffleHistory.value = []
      if (isShuffled.value) {
        initShufflePool(currentIndex.value)
      }
    }
    await PlayTrack(infoHash, filePath)
  }

  async function playNext() {
    if (playlist.value.length === 0) return
    if (playlist.value.length === 1) {
      currentIndex.value = 0
      const track = playlist.value[0]
      await PlayTrack(track.infoHash, track.filePath)
      return
    }

    let nextIdx: number
    if (isShuffled.value) {
      // Save current track to history so user can go back via playPrev
      if (currentIndex.value >= 0 && currentIndex.value < playlist.value.length) {
        shuffleHistory.value.push(currentIndex.value)
      }

      // Filter unplayed pool to valid bounds
      unplayedIndices.value = unplayedIndices.value.filter(i => i >= 0 && i < playlist.value.length && i !== currentIndex.value)

      // When pool is exhausted, re-fill with all tracks except current (start new non-repeating cycle)
      if (unplayedIndices.value.length === 0) {
        initShufflePool(currentIndex.value)
      }

      if (unplayedIndices.value.length > 0) {
        nextIdx = unplayedIndices.value.shift()!
      } else {
        nextIdx = (currentIndex.value + 1) % playlist.value.length
      }
    } else {
      nextIdx = (currentIndex.value + 1) % playlist.value.length
    }

    currentIndex.value = nextIdx
    const nextTrack = playlist.value[nextIdx]
    await PlayTrack(nextTrack.infoHash, nextTrack.filePath)
  }

  async function playPrev() {
    if (playlist.value.length === 0) return
    if (playlist.value.length === 1) {
      currentIndex.value = 0
      const track = playlist.value[0]
      await PlayTrack(track.infoHash, track.filePath)
      return
    }

    let prevIdx: number
    if (isShuffled.value) {
      if (shuffleHistory.value.length > 0) {
        prevIdx = shuffleHistory.value.pop()!
        // Return current track back to pool so next forward navigation doesn't skip it
        if (currentIndex.value >= 0 && currentIndex.value < playlist.value.length) {
          unplayedIndices.value.unshift(currentIndex.value)
        }
      } else {
        // No prior tracks in history: step back in playlist order
        prevIdx = (currentIndex.value - 1 + playlist.value.length) % playlist.value.length
      }
    } else {
      prevIdx = (currentIndex.value - 1 + playlist.value.length) % playlist.value.length
    }

    currentIndex.value = prevIdx
    const prevTrack = playlist.value[prevIdx]
    await PlayTrack(prevTrack.infoHash, prevTrack.filePath)
  }

  async function togglePause() {
    await Pause()
  }

  async function stop() {
    await Stop()
  }

  async function seek(seconds: number) {
    await SeekTo(seconds)
  }

  async function setVolume(v: number) {
    status.value.volume = v
    try {
      localStorage.setItem('torrent_player_volume', v.toString())
    } catch { /* ignore */ }
    await SetVolume(v)
  }

  function toggleRepeat() {
    if (repeatMode.value === 'off') {
      repeatMode.value = 'all'
    } else if (repeatMode.value === 'all') {
      repeatMode.value = 'one'
    } else {
      repeatMode.value = 'off'
    }
  }

  function toggleShuffle() {
    isShuffled.value = !isShuffled.value
    shuffleHistory.value = []
    if (isShuffled.value) {
      initShufflePool(currentIndex.value)
    } else {
      unplayedIndices.value = []
    }
    try {
      localStorage.setItem('torrent_player_shuffle', isShuffled.value.toString())
    } catch { /* ignore */ }
  }

  const isPlaying = computed(() => status.value.state === 'playing')
  const isPaused  = computed(() => status.value.state === 'paused')
  const currentQueueTrack = computed(() => {
    if (currentIndex.value >= 0 && currentIndex.value < playlist.value.length) {
      return playlist.value[currentIndex.value]
    }
    return null
  })

  return {
    status,
    isPlaying,
    isPaused,
    currentQueueTrack,
    repeatMode,
    isShuffled,
    playlist,
    currentIndex,
    isCoverFullscreen,
    openCoverFullscreen,
    closeCoverFullscreen,
    toggleCoverFullscreen,
    startPolling,
    stopPolling,
    play,
    playNext,
    playPrev,
    togglePause,
    stop,
    seek,
    setVolume,
    toggleRepeat,
    toggleShuffle
  }
})
