<script lang="ts" setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useDownloadsStore } from '../store/downloads'
import { usePlayerStore } from '../store/player'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'
import { GetDownloadDir, OpenDownloadFolder, OpenFileInFolder } from '../../wailsjs/go/main/App'

const props = defineProps<{
  active?: boolean
}>()

const downloads = useDownloadsStore()
const player = usePlayerStore()
const dialog = useDialogStore()
const { t } = useI18n()
const downloadDir = ref('')

watch(() => props.active, (isActive) => {
  if (isActive) {
    downloads.refreshProgress()
  }
})

// Stable sorted downloads list (newest first, tie-breaker by infoHash)
const sortedDownloads = computed(() => {
  return [...downloads.downloads].sort((a, b) => {
    const aTime = a.addedAt || 0
    const bTime = b.addedAt || 0
    if (aTime !== bTime) {
      return bTime - aTime
    }
    return a.infoHash.localeCompare(b.infoHash)
  })
})

// Expanded: infoHash → show file list
const expanded = ref<Record<string, boolean>>({})

onMounted(async () => {
  try {
    downloadDir.value = await GetDownloadDir()
  } catch {
    /* ignore */
  }
  await downloads.refreshProgress()
})

async function openFolder() {
  await OpenDownloadFolder()
}

async function openFile(infoHash: string, path: string) {
  await OpenFileInFolder(infoHash, path)
}

async function toggleExpand(infoHash: string) {
  expanded.value[infoHash] = !expanded.value[infoHash]
  if (expanded.value[infoHash] && !downloads.files[infoHash]) {
    await downloads.fetchFiles(infoHash)
  }
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 ** 2) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 ** 3) return (bytes / 1024 ** 2).toFixed(1) + ' MB'
  return (bytes / 1024 ** 3).toFixed(2) + ' GB'
}

function formatLimit(kbps: number) {
  if (!kbps || kbps <= 0) return ''
  if (kbps >= 1024) {
    const mb = kbps / 1024
    return (Number.isInteger(mb) ? mb : mb.toFixed(1)) + ' MB/s'
  }
  return kbps + ' KB/s'
}

function stateColor(state: string) {
  return { 
    downloading: '#2196f3', 
    seeding: '#4caf50', 
    paused: '#ff9800', 
    queued: '#9e9e9e',
    metadata: '#b388ff'
  }[state] ?? '#9e9e9e'
}

function stateLabel(state: string) {
  return { 
    downloading: '⬇ ' + t('downloads.statusDownloading'), 
    seeding: '🌱 ' + t('downloads.statusSeeding'), 
    paused: '⏸ ' + t('downloads.statusPaused'), 
    queued: '⏳ ' + t('downloads.statusQueued'),
    metadata: '⏳ ' + t('downloads.statusMetadata')
  }[state] ?? state
}
async function onAddFile(infoHash: string, file: any) {
  await downloads.addFile(infoHash, file.path)
}

async function onDeleteFile(infoHash: string, file: any) {
  const ok = await dialog.confirm({
    title: t('downloads.deleteFileTitle'),
    message: t('downloads.deleteFileMsg', { name: file.name }),
    details: t('downloads.deleteFileDetails'),
    confirmText: t('common.delete'),
    danger: true
  })
  if (ok) {
    await downloads.removeFile(infoHash, file.path)
  }
}

async function onDeleteTorrent(d: any) {
  const choice = await dialog.choice({
    title: t('downloads.deleteTorrentTitle'),
    message: t('downloads.deleteTorrentMsg', { name: d.name }),
    options: [
      { label: t('downloads.deleteWithFiles'), value: 'with_files', danger: true },
      { label: t('downloads.deleteKeepFiles'), value: 'keep_files' }
    ],
    cancelText: t('common.cancel')
  })
  if (!choice) return
  await downloads.remove(d.infoHash, choice === 'with_files')
}
</script>

<template>
  <div class="downloads-view">
    <div class="view-header">
      <div>
        <h1>{{ t('downloads.title') }}</h1>
        <div class="dl-path-hint" v-if="downloadDir" :title="downloadDir">
          📁 {{ t('downloads.saveFolder') }} <code>{{ downloadDir }}</code>
        </div>
      </div>
      <button class="btn btn-accent" @click="openFolder">
        📂 {{ t('downloads.openFolder') }}
      </button>
    </div>

    <div class="empty-state" v-if="!downloads.downloads.length">
      <div class="empty-icon">⬇️</div>
      <p>{{ t('downloads.emptyTitle') }}</p>
      <p class="empty-hint">{{ t('downloads.emptyHint') }}</p>
    </div>

    <div class="download-list" v-else>
      <div class="download-card" v-for="d in sortedDownloads" :key="d.infoHash">
        <div class="download-header">
          <div class="download-title-group">
            <div class="download-name" :title="d.name">{{ d.name }}</div>
            <div class="download-selected-track" v-if="d.selectedTrack" :title="d.selectedTrack">
              🎵 {{ t('downloads.selectedTrack') }} <span class="track-highlight">{{ d.selectedTrack }}</span>
            </div>
            <div class="download-selected-track" v-else-if="d.selectedCount && d.selectedCount > 1">
              🎵 {{ t('downloads.selectedTracks') }} <span class="track-highlight">{{ d.selectedCount }}</span>
            </div>
          </div>
          <div class="download-actions">
            <button
              class="ctrl"
              @click="downloads.pause(d.infoHash)"
              v-if="d.state === 'downloading' || d.state === 'seeding'"
              :title="d.state === 'seeding' ? t('downloads.stopSeeding') : t('downloads.pauseTorrent')"
            >
              ⏸
            </button>
            <button
              class="ctrl"
              @click="downloads.resume(d.infoHash)"
              v-if="d.state === 'paused'"
              :title="d.percentDone >= 100 ? t('downloads.resumeSeeding') : t('downloads.resumeTorrent')"
            >
              ▶
            </button>
            <button class="ctrl" @click="toggleExpand(d.infoHash)" :title="expanded[d.infoHash] ? t('downloads.collapseFiles') : t('downloads.showFiles')">
              📂 {{ expanded[d.infoHash] ? '▲' : '▼' }}
            </button>
            <button class="ctrl" @click="openFolder" :title="t('downloads.showInFolder')">📁</button>
            <button class="ctrl danger" @click="onDeleteTorrent(d)" :title="t('downloads.deleteBtn')">✕</button>
          </div>
        </div>

        <div class="progress-row">
          <div class="progress-bar" :class="{ 'is-metadata': d.state === 'metadata' }">
            <div
              class="progress-fill"
              :style="{
                width: d.state === 'metadata' ? '100%' : d.percentDone + '%',
                background: d.state === 'metadata' ? undefined : stateColor(d.state)
              }"
            ></div>
          </div>
          <span class="pct">{{ d.state === 'metadata' ? '...' : d.percentDone.toFixed(1) + '%' }}</span>
        </div>

        <div class="download-meta">
          <span :style="{ color: stateColor(d.state) }" class="state-badge">{{ stateLabel(d.state) }}</span>
          <span v-if="d.state === 'metadata'" class="meta-hint">{{ t('downloads.metadataHint') }}</span>
          <span v-else>{{ formatBytes(d.downloaded) }} / {{ formatBytes(d.total) }}</span>

          <span
            v-if="d.state !== 'metadata'"
            class="swarm-meta"
            :title="d.state === 'seeding' && (d.leechers ?? 0) === 0 ? t('downloads.seedingIdleTooltip') : undefined"
          >
            🌱 {{ d.seeders }} {{ t('downloads.seedersCount') }}
            <span class="meta-sep">·</span>
            👥 {{ d.leechers ?? 0 }} {{ t('downloads.leechersCount') }}
          </span>

          <span v-if="d.downloadSpeed > 0" class="speed-dl">
            ⬇ {{ formatBytes(d.downloadSpeed) }}/s
            <span v-if="(d.downloadLimitKBps ?? 0) > 0" class="speed-limit-badge">
              [{{ t('downloads.maxLimit') }} {{ formatLimit(d.downloadLimitKBps ?? 0) }}]
            </span>
          </span>
          <span
            v-if="d.uploadSpeed > 0 || d.state === 'seeding'"
            class="speed-ul"
            :class="{ 'speed-idle': d.uploadSpeed === 0 && d.state === 'seeding' }"
            :title="d.state === 'seeding' && d.uploadSpeed === 0 ? t('downloads.seedingIdleTooltip') : undefined"
          >
            ⬆ {{ formatBytes(d.uploadSpeed) }}/s<span v-if="d.state === 'seeding' && d.uploadSpeed === 0" class="idle-tag"> ({{ t('downloads.idle') }})</span>
            <span v-if="(d.uploadLimitKBps ?? 0) > 0" class="speed-limit-badge">
              [{{ t('downloads.maxLimit') }} {{ formatLimit(d.uploadLimitKBps ?? 0) }}]
            </span>
          </span>
        </div>

        <!-- File list (expanded) with individual download/delete actions -->
        <div class="file-list" v-if="expanded[d.infoHash]">
          <div v-if="!downloads.files[d.infoHash]" class="loading">{{ t('downloads.loadingFiles') }}</div>
          <div
            v-for="f in downloads.files[d.infoHash]"
            :key="f.path"
            class="file-item"
          >
            <span class="file-icon">🎵</span>
            <span class="file-name" :title="f.path">{{ f.name }}</span>
            <span class="file-size">{{ formatBytes(f.size) }}</span>

            <!-- Status Pill -->
            <div class="file-status-wrap">
              <span v-if="f.isComplete" class="file-status complete">✓ {{ t('downloads.fileDownloaded') }}</span>
              <span v-else-if="f.isSelected" class="file-status downloading">⬇ {{ (f.percentDone ?? 0).toFixed(0) }}%</span>
              <span v-else class="file-status not-downloaded">{{ t('downloads.fileNotDownloaded') }}</span>
            </div>

            <!-- Actions -->
            <div class="file-actions">
              <!-- Listen (streams or plays local file) -->
              <button class="ctrl-mini" @click="player.play(d.infoHash, f.path)" :title="t('common.play')">▶</button>
              
              <!-- Download if not yet complete/selected -->
              <button
                v-if="!f.isComplete && !f.isSelected"
                class="ctrl-mini btn-add"
                @click="onAddFile(d.infoHash, f)"
                :title="t('downloads.downloadThisTrack')"
              >
                ⬇ {{ t('downloads.download') }}
              </button>

              <!-- In explorer -->
              <button
                v-if="f.isComplete"
                class="ctrl-mini"
                @click="openFile(d.infoHash, f.path)"
                :title="t('downloads.showInFolder')"
              >
                📁
              </button>

              <!-- Delete / Cancel -->
              <button
                v-if="f.isComplete || f.isSelected"
                class="ctrl-mini btn-del"
                @click="onDeleteFile(d.infoHash, f)"
                :title="t('common.delete')"
              >
                🗑
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.downloads-view { display: flex; flex-direction: column; gap: 20px; }

.view-header {
  display: flex; justify-content: space-between; align-items: flex-start; gap: 16px;
}
.view-header h1 { font-size: 24px; font-weight: 700; }
.dl-path-hint {
  font-size: 12px; color: var(--text2); margin-top: 4px;
}
.dl-path-hint code {
  color: var(--accent); background: var(--bg3); padding: 2px 6px; border-radius: 4px;
}

.empty-state { text-align: center; padding: 60px 0; color: var(--text2); }
.empty-icon { font-size: 52px; margin-bottom: 16px; }
.empty-hint { font-size: 13px; margin-top: 6px; opacity: 0.6; }

.download-list { display: flex; flex-direction: column; gap: 12px; }

.download-card {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
  display: flex; flex-direction: column; gap: 10px;
}

.download-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
.download-title-group { display: flex; flex-direction: column; gap: 4px; flex: 1; min-width: 0; }
.download-name { font-size: 14px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.download-selected-track {
  font-size: 12px; color: var(--text2); display: flex; align-items: center; gap: 4px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.track-highlight { color: var(--accent); font-weight: 500; }
.download-actions { display: flex; gap: 6px; flex-shrink: 0; }

.ctrl { font-size: 14px; padding: 4px 8px; border-radius: 6px; opacity: 0.7; transition: opacity 0.1s, background 0.1s; }
.ctrl:hover { opacity: 1; background: var(--surface2); }
.ctrl.danger:hover { background: var(--danger); color: #fff; }

.progress-row { display: flex; align-items: center; gap: 10px; }
.progress-bar { flex: 1; height: 6px; background: var(--surface2); border-radius: 3px; overflow: hidden; }
.progress-fill { height: 100%; border-radius: 3px; transition: width 0.3s; }
.pct { font-size: 13px; color: var(--text2); width: 48px; text-align: right; font-variant-numeric: tabular-nums; }

.download-meta { display: flex; gap: 16px; font-size: 13px; color: var(--text2); flex-wrap: wrap; }
.state-badge { font-weight: 600; }
.speed-dl { color: #2196f3; font-weight: 500; font-variant-numeric: tabular-nums; }
.speed-ul { color: #4caf50; font-weight: 500; font-variant-numeric: tabular-nums; }
.speed-ul.speed-idle { color: var(--text2); opacity: 0.8; }
.idle-tag { font-size: 11px; opacity: 0.75; margin-left: 2px; }
.speed-limit-badge { font-size: 11px; opacity: 0.75; margin-left: 3px; color: var(--text2); }
.swarm-meta { display: inline-flex; align-items: center; gap: 5px; }
.meta-sep { opacity: 0.4; margin: 0 1px; }

/* File list */
.file-list { border-top: 1px solid var(--border); padding-top: 10px; display: flex; flex-direction: column; gap: 6px; }
.loading { font-size: 13px; color: var(--text2); padding: 8px 0; }

.file-item {
  display: grid;
  grid-template-columns: auto 1fr auto auto auto;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.04);
  transition: background 0.1s, border-color 0.1s;
}
.file-item:hover {
  background: var(--surface);
  border-color: var(--border);
}

.file-icon { font-size: 16px; }
.file-name { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.file-size { font-size: 12px; color: var(--text2); white-space: nowrap; }

.file-status-wrap { display: flex; align-items: center; }
.file-status {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 12px;
  white-space: nowrap;
}
.file-status.complete {
  color: var(--accent);
  background: rgba(29, 185, 84, 0.15);
}
.file-status.downloading {
  color: #2196f3;
  background: rgba(33, 150, 243, 0.15);
}
.file-status.not-downloaded {
  color: var(--text2);
  background: rgba(255, 255, 255, 0.05);
}

.file-actions { display: flex; gap: 6px; align-items: center; }
.ctrl-mini {
  font-size: 12px; padding: 4px 10px; background: var(--surface2); border-radius: 6px;
  cursor: pointer; border: none; color: var(--text);
  transition: all 0.1s ease;
}
.ctrl-mini:hover { background: var(--accent); color: #000; }

.ctrl-mini.btn-add {
  background: rgba(29, 185, 84, 0.15);
  color: var(--accent);
  font-weight: 500;
}
.ctrl-mini.btn-add:hover {
  background: var(--accent);
  color: #000;
}

.ctrl-mini.btn-del {
  color: var(--text2);
}
.ctrl-mini.btn-del:hover {
  background: var(--danger);
  color: #fff;
}

.progress-bar.is-metadata .progress-fill {
  background: linear-gradient(90deg, #7c4dff 0%, #b388ff 50%, #7c4dff 100%) !important;
  background-size: 200% 100% !important;
  animation: metadata-pulse 1.8s infinite linear;
}

@keyframes metadata-pulse {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.meta-hint {
  color: #b388ff;
  font-style: italic;
}
</style>
