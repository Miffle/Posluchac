import { watch } from 'vue'
import { usePlayerStore } from '../store/player'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export function setupMediaIntegration() {
  const player = usePlayerStore()

  // 1. Silent audio loop to activate Windows SMTC (System Media Transport Controls) in Chromium/WebView2
  const silentWav = 'data:audio/wav;base64,UklGRigAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQQAAAAAAA=='
  const silentAudio = new Audio(silentWav)
  silentAudio.loop = true
  silentAudio.volume = 0.001

  // 2. Media Session API handlers
  if ('mediaSession' in navigator) {
    try {
      navigator.mediaSession.setActionHandler('play', () => {
        player.togglePause()
      })
      navigator.mediaSession.setActionHandler('pause', () => {
        player.togglePause()
      })
      navigator.mediaSession.setActionHandler('nexttrack', () => {
        player.playNext()
      })
      navigator.mediaSession.setActionHandler('previoustrack', () => {
        player.playPrev()
      })
      navigator.mediaSession.setActionHandler('stop', () => {
        player.stop()
      })
    } catch (e) {
      console.warn('Failed to register mediaSession action handlers', e)
    }
  }

  // 3. Sync player state with MediaSession & Windows SMTC
  watch(
    () => [player.status.state, player.status.track, player.currentQueueTrack],
    () => {
      const state = player.status.state
      const track = player.status.track
      const qTrack = player.currentQueueTrack

      if (state === 'playing') {
        silentAudio.play().catch(() => {
          // Autoplay policy might delay silent play until first user interaction
        })

        if ('mediaSession' in navigator) {
          navigator.mediaSession.playbackState = 'playing'

          let title = track?.title || qTrack?.title || ''
          let artist = track?.artist || qTrack?.artist || ''
          const album = track?.album || qTrack?.album || ''
          const fn = track?.fileName || qTrack?.fileName || ''

          if (!title && fn) {
            const noExt = fn.replace(/\.[^/.]+$/, '')
            if (noExt.includes(' - ')) {
              artist = noExt.split(' - ')[0].trim()
              title = noExt.split(' - ').slice(1).join(' - ').trim()
            } else {
              title = noExt
            }
          }

          const coverPath = track?.coverPath || qTrack?.coverPath
          const artwork = coverPath
            ? [{ src: '/api/cover?path=' + encodeURIComponent(coverPath), sizes: '512x512', type: 'image/jpeg' }]
            : []

          try {
            navigator.mediaSession.metadata = new MediaMetadata({
              title: title || 'Posluchac',
              artist: artist || 'Posluchac',
              album: album || '',
              artwork
            })
          } catch (e) {
            console.warn('Failed to set media metadata', e)
          }
        }
      } else if (state === 'paused') {
        silentAudio.pause()
        if ('mediaSession' in navigator) {
          navigator.mediaSession.playbackState = 'paused'
        }
      } else {
        silentAudio.pause()
        if ('mediaSession' in navigator) {
          navigator.mediaSession.playbackState = 'none'
        }
      }
    },
    { deep: true, immediate: true }
  )

  // 4. Windows Global Hotkeys from Go backend (works even when minimized or in background)
  EventsOn('media:playpause', () => {
    player.togglePause()
  })

  EventsOn('media:next', () => {
    player.playNext()
  })

  EventsOn('media:prev', () => {
    player.playPrev()
  })

  // 5. In-window keyboard shortcuts (when app window has focus)
  window.addEventListener('keydown', (e: KeyboardEvent) => {
    const target = e.target as HTMLElement
    const tag = target?.tagName?.toLowerCase()
    if (tag === 'input' || tag === 'textarea' || target?.isContentEditable) {
      return
    }

    if (e.code === 'Space') {
      e.preventDefault()
      player.togglePause()
    } else if (e.code === 'MediaPlayPause' || e.key === 'MediaPlayPause') {
      e.preventDefault()
      player.togglePause()
    } else if (e.code === 'MediaTrackNext' || e.key === 'MediaTrackNext') {
      e.preventDefault()
      player.playNext()
    } else if (e.code === 'MediaTrackPrevious' || e.key === 'MediaTrackPrevious') {
      e.preventDefault()
      player.playPrev()
    } else if (e.code === 'MediaStop' || e.key === 'MediaStop') {
      e.preventDefault()
      player.stop()
    }
  })
}
