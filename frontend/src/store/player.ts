import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { GetStatus, Pause, Stop, SeekTo, SetVolume, PlayTrack, IsCurrentTrackSaved, SaveCurrentStreamedTrack } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useDialogStore } from './dialog'

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
  const isCurrentSaved = ref<boolean>(false)
  const isSavingTrack = ref<boolean>(false)

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

  async function checkIfCurrentSaved() {
    const tr = status.value.track
    if (!tr) {
      isCurrentSaved.value = false
      return
    }
    if (!tr.infoHash || tr.infoHash === 'local') {
      isCurrentSaved.value = true
      return
    }
    try {
      isCurrentSaved.value = await IsCurrentTrackSaved()
    } catch {
      isCurrentSaved.value = false
    }
  }

  async function saveCurrentTrack(): Promise<boolean> {
    if (isSavingTrack.value) return false
    isSavingTrack.value = true
    try {
      const res = await SaveCurrentStreamedTrack()
      if (res && res.id) {
        isCurrentSaved.value = true
        return true
      }
      return false
    } finally {
      isSavingTrack.value = false
    }
  }

  let eventsRegistered = false
  function startPolling() {
    if (!eventsRegistered) {
      eventsRegistered = true
      EventsOn('player:ended', () => {
        handleTrackEnded()
      })
    }

    pollInterval = setInterval(async () => {
      try {
        if (!volumeInitialized) {
          await SetVolume(status.value.volume)
          volumeInitialized = true
        }

        const s = (await GetStatus()) as PlayerStatus
        const prev = lastState
        lastState = s.state
        const trackChanged = (s.track?.filePath !== status.value.track?.filePath) || (s.track?.infoHash !== status.value.track?.infoHash)
        status.value = {
          ...s,
          track: s.track || status.value.track,
          volume: status.value.volume
        }

        if (trackChanged) {
          checkIfCurrentSaved()
        }

        // Track finished naturally (was playing, now stopped)
        if (prev === 'playing' && s.state === 'stopped') {
          handleTrackEnded()
        }
      } catch { /* ignore */ }
    }, 500)
  }

  let isHandlingEnd = false
  async function handleTrackEnded() {
    if (isHandlingEnd) return
    isHandlingEnd = true
    try {
      if (repeatMode.value === 'one' && status.value.track) {
        // Repeat same song
        await PlayTrack(status.value.track.infoHash, status.value.track.filePath)
      } else if (repeatMode.value === 'all') {
        // Repeat entire playlist (shuffled or sequential)
        await playNext()
      } else if (isShuffled.value) {
        // Shuffle mode with repeat OFF: only continue if there are unplayed tracks left
        if (unplayedIndices.value.length > 0) {
          await playNext()
        } else {
          await stop()
        }
      } else if (currentIndex.value >= 0 && currentIndex.value < playlist.value.length - 1) {
        // Sequential, repeat OFF: play next song until end of list
        await playNext()
      } else {
        // Reached end of playback queue: cleanly stop and reset
        await stop()
      }
    } finally {
      setTimeout(() => {
        isHandlingEnd = false
      }, 300)
    }
  }

  function stopPolling() {
    if (pollInterval) clearInterval(pollInterval)
  }

  async function play(infoHash: string, filePath: string, queue?: QueueTrack[]) {
    if (queue && queue.length > 0) {
      playlist.value = queue
      const idx = queue.findIndex(t => t.filePath === filePath || (t.infoHash === infoHash && t.fileName === filePath))
      currentIndex.value = idx !== -1 ? idx : 0
      shuffleHistory.value = []
      if (isShuffled.value) {
        initShufflePool(currentIndex.value)
      }
    } else {
      const idx = playlist.value.findIndex(t => t.filePath === filePath || (t.infoHash === infoHash && t.fileName === filePath))
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
    try {
      await PlayTrack(infoHash, filePath)
    } catch (err: any) {
      const msg = err?.message || String(err)
      console.warn('PlayTrack error:', msg)
      try {
        const dialog = useDialogStore()
        dialog.toast(msg, 'warning')
      } catch { /* ignore */ }
      status.value.state = 'stopped'
    }
  }

  async function playNext() {
    if (playlist.value.length === 0) return
    if (playlist.value.length === 1) {
      if (repeatMode.value === 'off') {
        await stop()
        return
      }
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

      // When pool is exhausted:
      if (unplayedIndices.value.length === 0) {
        if (repeatMode.value === 'all') {
          initShufflePool(currentIndex.value)
        } else {
          await stop()
          return
        }
      }

      if (unplayedIndices.value.length > 0) {
        nextIdx = unplayedIndices.value.shift()!
      } else {
        nextIdx = (currentIndex.value + 1) % playlist.value.length
      }
    } else {
      if (currentIndex.value >= playlist.value.length - 1) {
        if (repeatMode.value === 'off') {
          await stop()
          return
        }
      }
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
    if (status.value.state === 'stopped') {
      const track = currentQueueTrack.value || status.value.track
      if (track) {
        await play(track.infoHash, track.filePath)
        return
      }
      if (playlist.value.length > 0) {
        currentIndex.value = 0
        const first = playlist.value[0]
        await play(first.infoHash, first.filePath)
        return
      }
      return
    }
    await Pause()
  }

  async function stop() {
    await Stop()
    status.value.state = 'stopped'
    status.value.positionSec = 0
  }

  async function seek(seconds: number) {
    if (status.value.state === 'stopped') {
      const track = currentQueueTrack.value || status.value.track
      if (track) {
        await play(track.infoHash, track.filePath)
        await SeekTo(seconds)
        return
      }
    }
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
    toggleShuffle,
    isCurrentSaved,
    isSavingTrack,
    checkIfCurrentSaved,
    saveCurrentTrack
  }
})
