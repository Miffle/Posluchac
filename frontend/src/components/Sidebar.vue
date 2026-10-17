<script lang="ts" setup>
import { useUpdaterStore } from '../store/updater'

type View = 'search' | 'library' | 'downloads' | 'playlists'

defineProps<{ currentView: View }>()
const emit = defineEmits<{ navigate: [view: View] }>()

const updater = useUpdaterStore()

const items: { id: View; label: string; icon: string }[] = [
  { id: 'search',    label: 'Поиск',      icon: '🔍' },
  { id: 'library',   label: 'Библиотека', icon: '🎵' },
  { id: 'downloads', label: 'Загрузки',   icon: '⬇️' },
  { id: 'playlists', label: 'Плейлисты',  icon: '📋' },
]
</script>

<template>
  <aside class="sidebar">
    <div class="logo">
      <span class="logo-icon">🌊</span>
      <span class="logo-text">Posluchac</span>
    </div>

    <nav>
      <button
        v-for="item in items"
        :key="item.id"
        class="nav-item"
        :class="{ active: currentView === item.id }"
        @click="emit('navigate', item.id)"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span>{{ item.label }}</span>
      </button>
    </nav>

    <!-- Bottom section with update banner & version -->
    <div class="sidebar-bottom">
      <div
        class="update-banner"
        v-if="updater.updateInfo?.hasUpdate"
        @click="updater.showModal = true"
        title="Нажмите, чтобы посмотреть список изменений и обновиться"
      >
        <span class="banner-pulse"></span>
        <div class="banner-text">
          <div class="banner-title">Новая версия!</div>
          <div class="banner-ver">v{{ updater.updateInfo.latestVersion }}</div>
        </div>
        <span class="banner-arrow">➔</span>
      </div>

      <div class="version-row">
        <span class="version-tag" v-if="updater.currentVersion">v{{ updater.currentVersion }}</span>
        <button
          class="check-btn"
          @click="updater.check(true)"
          :disabled="updater.isChecking"
          title="Проверить наличие обновлений"
        >
          <span :class="['check-icon', { spinning: updater.isChecking }]">🔄</span>
        </button>
      </div>

      <div class="updater-status" v-if="updater.statusMessage">
        {{ updater.statusMessage }}
      </div>
      <div class="updater-error" v-if="updater.error">
        {{ updater.error }}
      </div>
    </div>
  </aside>
</template>

<style scoped>
.sidebar {
  grid-column: 1;
  grid-row: 1;
  background: var(--bg2);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 16px 12px 12px;
  gap: 4px;
  min-height: 0;
  overflow-y: auto;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 8px 24px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 12px;
}
.logo-icon { font-size: 22px; }
.logo-text { font-size: 16px; font-weight: 700; color: var(--accent); letter-spacing: 0.3px; }

nav { display: flex; flex-direction: column; gap: 2px; }

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text2);
  transition: background 0.12s, color 0.12s;
  text-align: left;
}
.nav-item:hover { background: var(--surface); color: var(--text); }
.nav-item.active { background: var(--surface); color: var(--accent); font-weight: 700; }
.nav-icon { font-size: 16px; width: 20px; text-align: center; }

/* Sidebar bottom */
.sidebar-bottom {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 6px 0;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.update-banner {
  background: linear-gradient(135deg, rgba(29, 185, 84, 0.22) 0%, rgba(29, 185, 84, 0.08) 100%);
  border: 1px solid var(--accent);
  border-radius: 8px;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.update-banner:hover {
  background: rgba(29, 185, 84, 0.28);
  transform: translateY(-1px);
}

.banner-pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent);
  flex-shrink: 0;
  animation: pulse 1.6s infinite ease-in-out;
}

@keyframes pulse {
  0% { transform: scale(0.9); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.9); opacity: 0.8; }
}

.banner-text {
  flex: 1;
  min-width: 0;
}
.banner-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.banner-ver {
  font-size: 12px;
  font-weight: 600;
  color: #ffffff;
}
.banner-arrow {
  font-size: 12px;
  color: var(--accent);
  font-weight: 700;
}

.version-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 2px;
}
.version-tag {
  font-size: 12px;
  color: var(--text2);
  font-family: monospace;
}
.check-btn {
  font-size: 13px;
  padding: 3px 6px;
  border-radius: 4px;
  opacity: 0.6;
  transition: all 0.12s;
  color: var(--text2);
  display: flex;
  align-items: center;
}
.check-btn:hover:not(:disabled) {
  opacity: 1;
  background: var(--surface);
  color: var(--text);
}
.check-icon.spinning {
  display: inline-block;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.updater-status {
  font-size: 11px;
  color: var(--accent);
  background: rgba(29, 185, 84, 0.1);
  padding: 4px 6px;
  border-radius: 4px;
  line-height: 1.3;
}
.updater-error {
  font-size: 11px;
  color: #ff8080;
  background: rgba(231, 76, 60, 0.1);
  padding: 4px 6px;
  border-radius: 4px;
  line-height: 1.3;
}
</style>
