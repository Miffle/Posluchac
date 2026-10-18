<script lang="ts" setup>
import { onMounted, computed } from 'vue'
import { useSettingsStore, type Language } from '../store/settings'
import { useUpdaterStore } from '../store/updater'
import { useI18n } from '../i18n'
import { OpenExternalURL } from '../../wailsjs/go/main/App'

const settings = useSettingsStore()
const updater = useUpdaterStore()
const { t } = useI18n()

onMounted(() => {
  settings.loadDownloadDir()
})

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

      <!-- 3. About Section -->
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
</style>
