<script lang="ts" setup>
import { onMounted, computed, ref, watch } from 'vue'
import { useSettingsStore, type Language, type LibraryFolder } from '../store/settings'
import { useUpdaterStore } from '../store/updater'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'
import { OpenExternalURL } from '../../wailsjs/go/main/App'

const settings = useSettingsStore()
const updater = useUpdaterStore()
const dialog = useDialogStore()
const { t } = useI18n()

const customDlInput = ref('')
const customUlInput = ref('')
const isCustomDl = ref(false)
const isCustomUl = ref(false)
const rescanMessage = ref('')

const dlPresets = computed(() => [
  { value: 0, label: t('settings.unlimited') },
  { value: 1024, label: '1 MB/s' },
  { value: 2048, label: '2 MB/s' },
  { value: 5120, label: '5 MB/s' },
  { value: 10240, label: '10 MB/s' },
])

const ulPresets = computed(() => [
  { value: 0, label: t('settings.unlimited') },
  { value: 256, label: '256 KB/s' },
  { value: 512, label: '512 KB/s' },
  { value: 1024, label: '1 MB/s' },
  { value: 2048, label: '2 MB/s' },
])

onMounted(async () => {
  settings.loadDownloadDir()
  await settings.loadSpeedLimits()
  await settings.loadLibraryFolders()
  await settings.loadDiscordRPC()
  syncCustomInputs()
})

async function handleAddFolder() {
  const added = await settings.addFolderDialog()
  if (added) {
    await handleRescan()
  }
}

async function confirmRemoveFolder(folder: LibraryFolder) {
  const selected = await dialog.choice({
    title: t('settings.removeFolderTitle'),
    message: t('settings.removeFolderMsg', { path: folder.path }),
    options: [
      { label: t('settings.removeFolderDeleteTracks'), value: 'delete_tracks', danger: true },
      { label: t('settings.removeFolderKeepTracks'), value: 'keep_tracks', primary: true }
    ]
  })
  if (!selected) return
  await settings.removeFolder(folder.id, selected === 'delete_tracks')
}

async function handleRescan() {
  rescanMessage.value = ''
  try {
    const count = await settings.rescanLibrary()
    rescanMessage.value = t('settings.rescanDone', { count })
  } catch (e: any) {
    dialog.toast(e?.message || String(e), 'error')
  }
}

function syncCustomInputs() {
  const currentDl = settings.downloadLimitKBps
  const isPresetDl = dlPresets.value.some(p => p.value === currentDl)
  isCustomDl.value = !isPresetDl && currentDl > 0
  if (isCustomDl.value) {
    customDlInput.value = String(currentDl)
  }

  const currentUl = settings.uploadLimitKBps
  const isPresetUl = ulPresets.value.some(p => p.value === currentUl)
  isCustomUl.value = !isPresetUl && currentUl > 0
  if (isCustomUl.value) {
    customUlInput.value = String(currentUl)
  }
}

async function selectDlPreset(val: number) {
  isCustomDl.value = false
  await settings.setSpeedLimits(val, settings.uploadLimitKBps)
}

function enableCustomDl() {
  isCustomDl.value = true
  if (!customDlInput.value && settings.downloadLimitKBps > 0) {
    customDlInput.value = String(settings.downloadLimitKBps)
  }
}

async function applyCustomDl() {
  const val = parseInt(customDlInput.value, 10)
  if (!isNaN(val) && val >= 0) {
    await settings.setSpeedLimits(val, settings.uploadLimitKBps)
  }
}

async function selectUlPreset(val: number) {
  isCustomUl.value = false
  await settings.setSpeedLimits(settings.downloadLimitKBps, val)
}

function enableCustomUl() {
  isCustomUl.value = true
  if (!customUlInput.value && settings.uploadLimitKBps > 0) {
    customUlInput.value = String(settings.uploadLimitKBps)
  }
}

async function applyCustomUl() {
  const val = parseInt(customUlInput.value, 10)
  if (!isNaN(val) && val >= 0) {
    await settings.setSpeedLimits(settings.downloadLimitKBps, val)
  }
}

const languages = computed<{ id: Language; flag: string; name: string; nativeName: string }[]>(() => [
  { id: 'ru', flag: '🇷🇺', name: t('settings.ru'), nativeName: t('settings.ruNative') },
  { id: 'en', flag: '🇬🇧', name: t('settings.en'), nativeName: t('settings.enNative') },
])

function openGithub() {
  OpenExternalURL('https://github.com/Miffle/Posluchac')
}
</script>

<template>
  <div class="settings-view">
    <div class="settings-header">
      <div class="title-row">
        <span class="settings-icon">⚙️</span>
        <div>
          <h1 class="settings-title">{{ t('settings.title') }}</h1>
          <p class="settings-subtitle">{{ t('settings.subtitle') }}</p>
        </div>
      </div>
    </div>

    <div class="settings-content">
      <!-- 1. Language Section -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">🌐</div>
          <div>
            <h2 class="card-title">{{ t('settings.languageTitle') }}</h2>
            <p class="card-desc">{{ t('settings.languageDesc') }}</p>
          </div>
        </div>

        <div class="lang-grid">
          <div
            v-for="item in languages"
            :key="item.id"
            class="lang-card"
            :class="{ active: settings.language === item.id }"
            @click="settings.setLanguage(item.id)"
          >
            <div class="lang-flag">{{ item.flag }}</div>
            <div class="lang-info">
              <span class="lang-name">{{ item.name }}</span>
              <span class="lang-sub">{{ item.nativeName }}</span>
            </div>
            <div class="lang-check" v-if="settings.language === item.id">
              <svg viewBox="0 0 24 24" class="check-svg">
                <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" fill="currentColor"/>
              </svg>
            </div>
          </div>
        </div>
      </section>

      <!-- 2. Storage & Downloads Section -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">📁</div>
          <div>
            <h2 class="card-title">{{ t('settings.storageTitle') }}</h2>
            <p class="card-desc">{{ t('settings.storageDesc') }}</p>
          </div>
        </div>

        <div class="storage-row">
          <div class="path-display" :title="settings.downloadDir || t('common.loading')">
            <span class="path-icon">📂</span>
            <span class="path-text">{{ settings.downloadDir || t('common.loading') }}</span>
          </div>
          <button class="action-btn" @click="settings.openFolder">
            <span>{{ t('settings.openFolder') }}</span>
          </button>
        </div>
      </section>

      <!-- 3. Library Folders Section -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">📚</div>
          <div>
            <h2 class="card-title">{{ t('settings.foldersTitle') }}</h2>
            <p class="card-desc">{{ t('settings.foldersDesc') }}</p>
          </div>
        </div>

        <div class="folders-list">
          <div
            v-for="folder in settings.libraryFolders"
            :key="folder.id"
            class="folder-item"
          >
            <div class="folder-path-wrap">
              <span class="folder-icon">📁</span>
              <span class="folder-path" :title="folder.path">{{ folder.path }}</span>
              <span v-if="folder.isDefault" class="folder-badge-default">{{ t('settings.defaultFolderBadge') }}</span>
            </div>
            <button
              v-if="!folder.isDefault"
              class="folder-delete-btn"
              :title="t('common.delete')"
              @click="confirmRemoveFolder(folder)"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
              </svg>
            </button>
          </div>
        </div>

        <div class="folders-actions">
          <button class="action-btn" @click="handleAddFolder">
            <span>{{ t('settings.addFolderBtn') }}</span>
          </button>
          <button
            class="secondary-btn"
            @click="handleRescan"
            :disabled="settings.isScanning"
          >
            <span :class="['btn-icon', { spinning: settings.isScanning }]">🔄</span>
            <span>{{ settings.isScanning ? t('settings.rescanning') : t('settings.rescanBtn') }}</span>
          </button>
        </div>

        <div v-if="rescanMessage" class="rescan-status-box">
          {{ rescanMessage }}
        </div>
      </section>

      <!-- 4. BitTorrent Bandwidth Limits -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">⚡</div>
          <div>
            <h2 class="card-title">{{ t('settings.bandwidthTitle') }}</h2>
            <p class="card-desc">{{ t('settings.bandwidthDesc') }}</p>
          </div>
        </div>

        <div class="limits-group">
          <!-- Download limit -->
          <div class="limit-row">
            <span class="limit-label">⬇ {{ t('settings.downloadLimit') }}</span>
            <div class="preset-pills">
              <button
                v-for="p in dlPresets"
                :key="p.value"
                class="pill-btn"
                :class="{ active: !isCustomDl && settings.downloadLimitKBps === p.value }"
                @click="selectDlPreset(p.value)"
              >
                {{ p.label }}
              </button>
              <button
                class="pill-btn"
                :class="{ active: isCustomDl }"
                @click="enableCustomDl"
              >
                {{ t('settings.customLimit') }}
              </button>
            </div>
            <div class="custom-input-row" v-if="isCustomDl">
              <input
                type="number"
                min="0"
                v-model="customDlInput"
                :placeholder="t('settings.customPlaceholder')"
                class="custom-input"
                @keyup.enter="applyCustomDl"
              />
              <span class="unit-label">KB/s</span>
              <button class="action-btn-sm" @click="applyCustomDl">{{ t('common.save') }}</button>
            </div>
          </div>

          <!-- Upload limit -->
          <div class="limit-row">
            <span class="limit-label">⬆ {{ t('settings.uploadLimit') }}</span>
            <div class="preset-pills">
              <button
                v-for="p in ulPresets"
                :key="p.value"
                class="pill-btn"
                :class="{ active: !isCustomUl && settings.uploadLimitKBps === p.value }"
                @click="selectUlPreset(p.value)"
              >
                {{ p.label }}
              </button>
              <button
                class="pill-btn"
                :class="{ active: isCustomUl }"
                @click="enableCustomUl"
              >
                {{ t('settings.customLimit') }}
              </button>
            </div>
            <div class="custom-input-row" v-if="isCustomUl">
              <input
                type="number"
                min="0"
                v-model="customUlInput"
                :placeholder="t('settings.customPlaceholder')"
                class="custom-input"
                @keyup.enter="applyCustomUl"
              />
              <span class="unit-label">KB/s</span>
              <button class="action-btn-sm" @click="applyCustomUl">{{ t('common.save') }}</button>
            </div>
          </div>
        </div>
      </section>

      <!-- 5. Discord Rich Presence Section -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">🎮</div>
          <div>
            <h2 class="card-title">{{ t('settings.discordTitle') }}</h2>
            <p class="card-desc">{{ t('settings.discordDesc') }}</p>
          </div>
        </div>

        <div class="toggle-row">
          <span class="toggle-label">{{ t('settings.discordToggle') }}</span>
          <label class="switch">
            <input
              type="checkbox"
              :checked="settings.discordRPCEnabled"
              @change="settings.setDiscordRPC(($event.target as HTMLInputElement).checked)"
            />
            <span class="slider round"></span>
          </label>
        </div>
      </section>

      <!-- 6. About Section -->
      <section class="settings-card">
        <div class="card-header">
          <div class="card-icon">ℹ️</div>
          <div>
            <h2 class="card-title">{{ t('settings.aboutTitle') }}</h2>
            <p class="card-desc">{{ t('settings.aboutDesc') }}</p>
          </div>
        </div>

        <div class="about-grid">
          <div class="about-item">
            <span class="about-label">{{ t('settings.version') }}:</span>
            <span class="about-value version-badge">v{{ updater.currentVersion || '1.1.0' }}</span>
          </div>
          <div class="about-item">
            <span class="about-label">{{ t('settings.developer') }}:</span>
            <span class="about-value">Roman (Miffle)</span>
          </div>
        </div>

        <div class="about-actions">
          <button
            class="action-btn check-updates-btn"
            @click="updater.check(true)"
            :disabled="updater.isChecking"
          >
            <span :class="['btn-icon', { spinning: updater.isChecking }]">🔄</span>
            <span>{{ updater.isChecking ? t('settings.checkingUpdate') : t('settings.checkUpdateBtn') }}</span>
          </button>

          <button class="secondary-btn" @click="openGithub">
            <span class="btn-icon">🔗</span>
            <span>{{ t('settings.githubRepo') }}</span>
          </button>
        </div>

        <div class="updater-status-box" v-if="updater.statusMessage">
          {{ updater.statusMessage }}
        </div>
        <div class="updater-error-box" v-if="updater.error">
          {{ updater.error }}
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  max-width: 860px;
  margin: 0 auto;
  padding-bottom: 40px;
}

.settings-header {
  margin-bottom: 28px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 20px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.settings-icon {
  font-size: 34px;
}

.settings-title {
  font-size: 26px;
  font-weight: 700;
  color: var(--text);
  letter-spacing: -0.3px;
}

.settings-subtitle {
  font-size: 14px;
  color: var(--text2);
  margin-top: 2px;
}

.settings-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.settings-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 22px;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
}

.card-header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  margin-bottom: 18px;
}

.card-icon {
  font-size: 22px;
  line-height: 1;
  padding-top: 2px;
}

.card-title {
  font-size: 17px;
  font-weight: 600;
  color: var(--text);
}

.card-desc {
  font-size: 13px;
  color: var(--text2);
  margin-top: 3px;
  line-height: 1.4;
}

/* Language selector */
.lang-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 14px;
}

.lang-card {
  background: var(--bg3);
  border: 2px solid transparent;
  border-radius: 10px;
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 14px;
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
}

.lang-card:hover {
  background: var(--surface);
  border-color: rgba(255, 255, 255, 0.12);
}

.lang-card.active {
  background: rgba(29, 185, 84, 0.08);
  border-color: var(--accent);
}

.lang-flag {
  font-size: 28px;
  line-height: 1;
}

.lang-info {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.lang-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.lang-sub {
  font-size: 12px;
  color: var(--text2);
}

.lang-check {
  width: 22px;
  height: 22px;
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
}

.check-svg {
  width: 20px;
  height: 20px;
}

/* Storage row */
.storage-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.path-display {
  flex: 1;
  min-width: 240px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-family: monospace;
  font-size: 13px;
  color: var(--text);
  overflow: hidden;
}

.path-icon {
  font-size: 15px;
  flex-shrink: 0;
}

.path-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Buttons */
.action-btn {
  background: var(--accent);
  color: #000;
  border: none;
  border-radius: 8px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.action-btn:hover:not(:disabled) {
  background: #22d460;
  transform: translateY(-1px);
}

.action-btn:active:not(:disabled) {
  transform: translateY(0);
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.secondary-btn {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px 18px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  transition: all 0.15s ease;
}

.secondary-btn:hover {
  background: var(--surface2);
  border-color: rgba(255, 255, 255, 0.15);
}

/* About items */
.about-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 20px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px 16px;
}

.about-item {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}

.about-label {
  color: var(--text2);
  min-width: 110px;
}

.about-value {
  color: var(--text);
  font-weight: 500;
}

.version-badge {
  background: rgba(29, 185, 84, 0.15);
  color: var(--accent);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: monospace;
  font-weight: 600;
}

.about-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.btn-icon {
  font-size: 15px;
  display: inline-block;
}

.btn-icon.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.updater-status-box {
  margin-top: 14px;
  padding: 8px 12px;
  background: rgba(29, 185, 84, 0.1);
  color: var(--accent);
  border-radius: 6px;
  font-size: 13px;
}

.updater-error-box {
  margin-top: 14px;
  padding: 8px 12px;
  background: rgba(231, 76, 60, 0.15);
  color: #ff6b6b;
  border-radius: 6px;
  font-size: 13px;
}

.limits-group {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.limit-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.limit-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.preset-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pill-btn {
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.pill-btn:hover {
  border-color: var(--accent);
  color: var(--text);
}

.pill-btn.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #000;
  font-weight: 600;
}

.custom-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.custom-input {
  width: 130px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 10px;
  color: var(--text);
  font-size: 13px;
}

.custom-input:focus {
  outline: none;
  border-color: var(--accent);
}

.unit-label {
  font-size: 13px;
  color: var(--text2);
}

.action-btn-sm {
  background: var(--accent);
  color: #000;
  border: none;
  border-radius: 6px;
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
}

.action-btn-sm:hover {
  opacity: 0.9;
}

/* Library Folders */
.folders-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.folder-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 12px;
}

.folder-path-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  overflow: hidden;
  flex: 1;
}

.folder-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.folder-path {
  font-size: 13px;
  color: var(--text);
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.folder-badge-default {
  background: rgba(29, 185, 84, 0.15);
  color: var(--accent);
  border: 1px solid rgba(29, 185, 84, 0.3);
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  flex-shrink: 0;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.folder-delete-btn {
  background: transparent;
  border: none;
  color: var(--text2);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.folder-delete-btn:hover {
  color: #ff6b6b;
  background: rgba(231, 76, 60, 0.1);
}

.folders-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.rescan-status-box {
  margin-top: 14px;
  padding: 8px 12px;
  background: rgba(29, 185, 84, 0.1);
  color: var(--accent);
  border-radius: 6px;
  font-size: 13px;
}

/* Toggle Switch for Discord RPC */
.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
}

.toggle-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.switch {
  position: relative;
  display: inline-block;
  width: 48px;
  height: 26px;
  flex-shrink: 0;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--surface2);
  transition: 0.25s ease;
  border-radius: 26px;
  border: 1px solid var(--border);
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: #ffffff;
  transition: 0.25s ease;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

input:checked + .slider {
  background-color: var(--accent);
  border-color: var(--accent);
}

input:checked + .slider:before {
  transform: translateX(22px);
  background-color: #000000;
}

</style>
