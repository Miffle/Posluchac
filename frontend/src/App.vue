<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import SearchView from './components/SearchView.vue'
import LibraryView from './components/LibraryView.vue'
import PlayerBar from './components/PlayerBar.vue'
import DownloadsView from './components/DownloadsView.vue'
import PlaylistView from './components/PlaylistView.vue'
import SettingsView from './components/SettingsView.vue'
import FullscreenCover from './components/FullscreenCover.vue'
import UpdateModal from './components/UpdateModal.vue'
import GlobalModal from './components/GlobalModal.vue'
import ToastContainer from './components/ToastContainer.vue'
import { usePlayerStore } from './store/player'
import { useDownloadsStore } from './store/downloads'
import { useUpdaterStore } from './store/updater'
import { setupMediaIntegration } from './utils/mediaSession'

import { provide } from 'vue'

type View = 'search' | 'library' | 'downloads' | 'playlists' | 'settings'
const currentView = ref<View>('library')

function navigate(view: View) {
  currentView.value = view
}
provide('navigate', navigate)

const playerStore = usePlayerStore()
const downloadsStore = useDownloadsStore()
const updaterStore = useUpdaterStore()

onMounted(() => {
  playerStore.startPolling()
  downloadsStore.startPolling()
  updaterStore.init()
  setupMediaIntegration()
})

</script>

<template>
  <div class="app">
    <Sidebar :current-view="currentView" @navigate="currentView = $event" />

    <main class="main-content">
      <SearchView   v-show="currentView === 'search'"    :active="currentView === 'search'" />
      <LibraryView  v-show="currentView === 'library'"   :active="currentView === 'library'" />
      <DownloadsView v-show="currentView === 'downloads'" :active="currentView === 'downloads'" />
      <PlaylistView v-show="currentView === 'playlists'" />
      <SettingsView v-show="currentView === 'settings'" />
    </main>

    <PlayerBar />
    <FullscreenCover />
    <UpdateModal />
    <GlobalModal />
    <ToastContainer />
  </div>
</template>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg:         #121212;
  --bg2:        #1a1a1a;
  --bg3:        #242424;
  --surface:    #2a2a2a;
  --surface2:   #333333;
  --accent:     #1db954;
  --accent-dim: #158a3e;
  --text:       #e8e8e8;
  --text2:      #a0a0a0;
  --danger:     #e74c3c;
  --border:     #333;
  --player-h:   88px;
  --sidebar-w:  220px;
}

body { background: var(--bg); color: var(--text); font-family: 'Nunito', system-ui, sans-serif; }

.app {
  display: grid;
  grid-template-columns: var(--sidebar-w) 1fr;
  grid-template-rows: 1fr var(--player-h);
  height: 100vh;
  overflow: hidden;
}

.main-content {
  grid-column: 2;
  grid-row: 1;
  overflow-y: auto;
  padding: 28px 32px;
  background: var(--bg);
}

::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--surface2); border-radius: 3px; }

input, button { font-family: inherit; }

button {
  cursor: pointer;
  border: none;
  background: none;
  color: var(--text);
}

.btn {
  padding: 8px 18px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  transition: background 0.15s, transform 0.1s;
}
.btn:active { transform: scale(0.97); }
.btn-accent { background: var(--accent); color: #000; }
.btn-accent:hover { background: #22d460; }
.btn-ghost { background: var(--surface); color: var(--text); }
.btn-ghost:hover { background: var(--surface2); }
.btn-danger { background: var(--danger); color: #fff; }

input[type="text"], input[type="password"] {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  padding: 9px 14px;
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}
input[type="text"]:focus, input[type="password"]:focus {
  border-color: var(--accent);
}
</style>
