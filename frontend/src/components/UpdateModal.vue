<script lang="ts" setup>
import { useUpdaterStore } from '../store/updater'
import { useI18n } from '../i18n'

const updater = useUpdaterStore()
const { t } = useI18n()

function formatSize(bytes: number): string {
  if (!bytes) return ''
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(1)} MB`
}
</script>

<template>
  <div class="modal-backdrop" v-if="updater.showModal" @click.self="updater.showModal = false">
    <div class="update-modal">
      <div class="modal-header">
        <div class="header-icon">🚀</div>
        <div class="header-text">
          <h3>{{ t('updater.title') }}</h3>
          <div class="version-badge-row">
            <span class="ver-curr">v{{ updater.currentVersion }}</span>
            <span class="ver-arrow">➔</span>
            <span class="ver-new">v{{ updater.updateInfo?.latestVersion }}</span>
            <span class="pub-date" v-if="updater.updateInfo?.publishedAt">{{ t('updater.from') }} {{ updater.updateInfo.publishedAt }}</span>
          </div>
        </div>
        <button class="close-btn" @click="updater.showModal = false" :disabled="updater.isUpdating">✕</button>
      </div>

      <div class="modal-body">
        <div class="release-title" v-if="updater.updateInfo?.releaseName">
          {{ updater.updateInfo.releaseName }}
        </div>

        <div class="changelog-box" v-if="updater.updateInfo?.releaseNotes">
          <div class="changelog-label">{{ t('updater.whatsNew') }}</div>
          <pre class="changelog-text">{{ updater.updateInfo.releaseNotes }}</pre>
        </div>

        <div class="download-size" v-if="updater.updateInfo?.assetSize">
          {{ t('updater.downloadSize') }} {{ formatSize(updater.updateInfo.assetSize) }}
        </div>

        <div class="updating-progress" v-if="updater.isUpdating">
          <div class="spinner"></div>
          <p style="white-space: pre-line;">{{ t('updater.downloadingAndRestart') }}</p>
        </div>

        <div class="alert-error" v-if="updater.error">
          ⚠️ {{ updater.error }}
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-ghost" @click="updater.openRelease()" :disabled="updater.isUpdating">
          🌐 {{ t('updater.githubPage') }}
        </button>
        <div class="footer-right">
          <button class="btn btn-ghost" @click="updater.showModal = false" :disabled="updater.isUpdating">
            {{ t('updater.later') }}
          </button>
          <button class="btn btn-accent btn-update" @click="updater.apply()" :disabled="updater.isUpdating">
            <span v-if="updater.isUpdating">{{ t('updater.updating') }}</span>
            <span v-else>🔄 {{ t('updater.updateNow') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.update-modal {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 14px;
  width: 100%;
  max-width: 520px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.6);
  animation: modal-in 0.18s ease-out;
}

@keyframes modal-in {
  from { opacity: 0; transform: scale(0.95) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border);
}

.header-icon {
  font-size: 28px;
}

.header-text {
  flex: 1;
}

.header-text h3 {
  font-size: 17px;
  font-weight: 700;
  margin-bottom: 4px;
}

.version-badge-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.ver-curr {
  color: var(--text2);
}

.ver-arrow {
  color: var(--accent);
  font-weight: 700;
}

.ver-new {
  color: var(--accent);
  font-weight: 700;
  background: rgba(29, 185, 84, 0.15);
  padding: 2px 6px;
  border-radius: 4px;
}

.pub-date {
  color: var(--text2);
  opacity: 0.8;
}

.close-btn {
  font-size: 16px;
  color: var(--text2);
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.1s;
}
.close-btn:hover:not(:disabled) {
  background: var(--surface);
  color: var(--text);
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 60vh;
  overflow-y: auto;
}

.release-title {
  font-size: 15px;
  font-weight: 700;
}

.changelog-box {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px;
}

.changelog-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text2);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.changelog-text {
  font-family: inherit;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text);
  max-height: 180px;
  overflow-y: auto;
}

.download-size {
  font-size: 12px;
  color: var(--text2);
}

.updating-progress {
  display: flex;
  align-items: center;
  gap: 14px;
  background: var(--surface);
  padding: 14px 16px;
  border-radius: 8px;
  border: 1px solid var(--accent);
}

.updating-progress p {
  font-size: 13px;
  line-height: 1.4;
  color: var(--text);
}

.spinner {
  width: 22px;
  height: 22px;
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.alert-error {
  background: rgba(231, 76, 60, 0.15);
  border: 1px solid var(--danger);
  color: #ff8080;
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 13px;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-top: 1px solid var(--border);
  background: var(--bg);
}

.footer-right {
  display: flex;
  gap: 10px;
}

.btn-update {
  font-weight: 700;
}
</style>
