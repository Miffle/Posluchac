<script lang="ts" setup>
import { ref, computed, watch, onMounted, inject } from 'vue'
import {
  Search,
  AddSource,
  DeleteSource,
  ListSources,
  SyncSources,
  PlayTrack
} from '../../wailsjs/go/main/App'
import { useDownloadsStore, type TorrentFile } from '../store/downloads'
import { usePlayerStore } from '../store/player'
import { useDialogStore } from '../store/dialog'

const navigate = inject<(view: string) => void>('navigate', () => {})

interface SearchResult {
  title: string
  magnetUri: string
  torrentUrl: string
  seeders: number
  leechers: number
  size: string
  source: string
  category: string
  infoHash?: string
}

interface Source {
  id: number
  name: string
  url: string
  itemCount: number
  updatedAt: string
}

export interface FolderGroup {
  key: string
  displayName: string
  files: TorrentFile[]
  audioFiles: TorrentFile[]
  coverFiles: TorrentFile[]
  hasCover: boolean
  coverName: string
  totalSize: number
}

const query = ref('')
const results = ref<SearchResult[]>([])
const loading = ref(false)
const error = ref('')

// Sources Manager (Hydra Launcher style)
const showSourcesModal = ref(false)
const sourcesList = ref<Source[]>([])
const newSourceUrl = ref('')
const addingSource = ref(false)
const sourceError = ref('')

// Magnet input
const showMagnetInput = ref(false)
const magnetUri = ref('')

// File selection modal
const showFileModal = ref(false)
const loadingFiles = ref(false)
const pendingInfoHash = ref('')
const pendingTitle = ref('')
const torrentFiles = ref<TorrentFile[]>([])
const selectedFiles = ref<Record<string, boolean>>({})
const fileSearch = ref('')
const audioOnlyFilter = ref(true)
const collapsedFolders = ref<Record<string, boolean>>({})

const downloadsStore = useDownloadsStore()
const playerStore = usePlayerStore()
const dialog = useDialogStore()

onMounted(async () => {
  await loadSources()
})

async function loadSources() {
  try {
    const res = await ListSources()
    sourcesList.value = (res || []) as Source[]
  } catch {
    /* ignore */
  }
}

async function addSourceHandler() {
  if (!newSourceUrl.value.trim()) return
  addingSource.value = true
  sourceError.value = ''
  try {
    await AddSource(newSourceUrl.value.trim())
    newSourceUrl.value = ''
    await loadSources()
  } catch (e: any) {
    sourceError.value = e?.message || 'Ошибка загрузки источника'
  } finally {
    addingSource.value = false
  }
}

async function removeSourceHandler(id: number) {
  const ok = await dialog.confirm({
    title: 'Удаление источника',
    message: 'Удалить этот источник поиска?',
    confirmText: 'Удалить',
    danger: true
  })
  if (!ok) return
  await DeleteSource(id)
  await loadSources()
}

async function syncSourcesHandler() {
  addingSource.value = true
  try {
    await SyncSources()
    await loadSources()
  } finally {
    addingSource.value = false
  }
}

async function doSearch() {
  if (!query.value.trim()) return
  loading.value = true
  error.value = ''
  results.value = []
  try {
    const res = await Search(query.value.trim())
    results.value = (res || []) as SearchResult[]
  } catch (e: any) {
    error.value = e?.message || 'Ошибка поиска'
  } finally {
    loading.value = false
  }
}

// Click on search result -> fetch file list and open selection modal
async function openFileSelector(result: SearchResult) {
  loadingFiles.value = true
  pendingTitle.value = result.title
  showFileModal.value = true
  torrentFiles.value = []
  selectedFiles.value = {}
  fileSearch.value = ''

  try {
    const pending = await downloadsStore.addFromResultPending(result)
    pendingInfoHash.value = pending.infoHash
    torrentFiles.value = pending.files || []

    // Start with empty selection so user picks only what they want
    selectedFiles.value = {}

    // Keep all folders expanded by default so user immediately sees all songs
    collapsedFolders.value = {}
  } catch (e: any) {
    showFileModal.value = false
    await dialog.alert({
      title: 'Ошибка получения файлов',
      message: 'Не удалось получить список файлов раздачи: ' + (e?.message || e),
      type: 'error'
    })
  } finally {
    loadingFiles.value = false
  }
}

function closeFilePicker() {
  if (pendingInfoHash.value) {
    downloadsStore.cancelPendingTorrent(pendingInfoHash.value)
    pendingInfoHash.value = ''
  }
  showFileModal.value = false
}

async function addMagnetManual() {
  if (!magnetUri.value.trim()) return
  try {
    await downloadsStore.addMagnet(magnetUri.value.trim())
    magnetUri.value = ''
    showMagnetInput.value = false
    dialog.toast('Торрент успешно добавлен в загрузки!', 'success')
  } catch (e: any) {
    await dialog.alert({
      title: 'Ошибка добавления торрента',
      message: e?.message || String(e),
      type: 'error'
    })
  }
}

// File modal helpers
function isAudio(name: string): boolean {
  const ext = name.slice(name.lastIndexOf('.')).toLowerCase()
  return ['.mp3', '.flac', '.ogg', '.wav', '.m4a', '.aac', '.opus', '.ape', '.wv'].includes(ext)
}

function isCover(name: string): boolean {
  const ext = name.slice(name.lastIndexOf('.')).toLowerCase()
  return ['.jpg', '.jpeg', '.png', '.webp'].includes(ext)
}

function normPath(p: string): string {
  return p.replace(/\\/g, '/')
}

const rootPrefix = computed(() => {
  if (torrentFiles.value.length === 0) return ''
  const first = normPath(torrentFiles.value[0].path)
  const slashIdx = first.indexOf('/')
  if (slashIdx === -1) return ''
  const top = first.substring(0, slashIdx + 1)
  const allMatch = torrentFiles.value.every(f => normPath(f.path).startsWith(top))
  return allMatch ? top : ''
})

const folderGroups = computed<FolderGroup[]>(() => {
  const map = new Map<string, {
    key: string
    displayName: string
    allFiles: TorrentFile[]
    coverFiles: TorrentFile[]
  }>()

  // First pass: collect files by folder
  for (const f of torrentFiles.value) {
    const p = normPath(f.path)
    const lastSlash = p.lastIndexOf('/')
    const folderKey = lastSlash !== -1 ? p.substring(0, lastSlash) : ''

    let displayName = folderKey
    if (folderKey === '') {
      displayName = 'Файлы раздачи'
    } else if (rootPrefix.value && folderKey.startsWith(rootPrefix.value.slice(0, -1))) {
      const rel = folderKey.substring(rootPrefix.value.length)
      displayName = rel || rootPrefix.value.slice(0, -1)
    }

    if (!map.has(folderKey)) {
      map.set(folderKey, {
        key: folderKey,
        displayName: displayName || folderKey,
        allFiles: [],
        coverFiles: []
      })
    }

    const group = map.get(folderKey)!
    group.allFiles.push(f)
    if (isCover(f.name)) {
      group.coverFiles.push(f)
    }
  }

  // Fallback: if folder has no cover, check parent folder ONLY for sub-discs (e.g. CD1 inside album)
  for (const [key, group] of map.entries()) {
    if (group.coverFiles.length === 0 && key !== '') {
      const parentSlash = key.lastIndexOf('/')
      if (parentSlash !== -1) {
        const baseName = key.substring(parentSlash + 1).toLowerCase()
        if (baseName.startsWith('cd') || baseName.startsWith('disc')) {
          const parentKey = key.substring(0, parentSlash)
          const parentGroup = map.get(parentKey)
          if (parentGroup && parentGroup.coverFiles.length > 0) {
            group.coverFiles = [...parentGroup.coverFiles]
          }
        }
      }
    }
  }

  // Second pass: filter by search & audioOnly, compute totals
  const groups: FolderGroup[] = []
  const q = fileSearch.value.trim().toLowerCase()

  for (const item of map.values()) {
    let visibleFiles = item.allFiles

    if (audioOnlyFilter.value) {
      visibleFiles = visibleFiles.filter(f => isAudio(f.name))
    }

    if (q) {
      visibleFiles = visibleFiles.filter(
        f => f.name.toLowerCase().includes(q) || item.displayName.toLowerCase().includes(q)
      )
    }

    if (visibleFiles.length > 0) {
      const audioFiles = item.allFiles.filter(f => isAudio(f.name))
      const totalSize = visibleFiles.reduce((sum, f) => sum + f.size, 0)
      const hasCover = item.coverFiles.length > 0
      const coverName = hasCover ? item.coverFiles[0].name : ''

      groups.push({
        key: item.key,
        displayName: item.displayName,
        files: visibleFiles,
        audioFiles,
        coverFiles: item.coverFiles,
        hasCover,
        coverName,
        totalSize
      })
    }
  }

  return groups
})

watch(fileSearch, () => {
  // Whenever search query changes, expand folders so songs are immediately visible
  collapsedFolders.value = {}
})

function toggleCollapse(key: string) {
  collapsedFolders.value[key] = !collapsedFolders.value[key]
}

function expandAllFolders() {
  for (const g of folderGroups.value) {
    collapsedFolders.value[g.key] = false
  }
}

function collapseAllFolders() {
  for (const g of folderGroups.value) {
    collapsedFolders.value[g.key] = true
  }
}

function isFolderSelected(group: FolderGroup): boolean {
  if (group.files.length === 0) return false
  return group.files.every(f => !!selectedFiles.value[f.path])
}

function isFolderIndeterminate(group: FolderGroup): boolean {
  if (group.files.length === 0) return false
  const count = group.files.filter(f => !!selectedFiles.value[f.path]).length
  return count > 0 && count < group.files.length
}

function toggleFolderSelection(group: FolderGroup) {
  const allSelected = isFolderSelected(group)
  for (const f of group.files) {
    selectedFiles.value[f.path] = !allSelected
  }
}

function getFolderSelectedCount(group: FolderGroup): number {
  return group.files.filter(f => !!selectedFiles.value[f.path]).length
}

const selectedCount = computed(() => {
  return Object.values(selectedFiles.value).filter(Boolean).length
})

const totalAudioFilesCount = computed(() => {
  return torrentFiles.value.filter(f => isAudio(f.name)).length
})

const selectedSize = computed(() => {
  let total = 0
  for (const f of torrentFiles.value) {
    if (selectedFiles.value[f.path]) {
      total += f.size
    }
  }
  return formatBytes(total)
})

const totalFilesSize = computed(() => {
  const sum = torrentFiles.value.reduce((acc, f) => acc + f.size, 0)
  return formatBytes(sum)
})

function selectAllFiltered() {
  for (const group of folderGroups.value) {
    for (const f of group.files) {
      selectedFiles.value[f.path] = true
    }
  }
}

function deselectAll() {
  selectedFiles.value = {}
}

async function confirmDownloadSelected() {
  const chosenPaths = new Set(
    Object.entries(selectedFiles.value)
      .filter(([_, sel]) => sel)
      .map(([path]) => path)
  )

  if (chosenPaths.size === 0) {
    dialog.toast('Выберите хотя бы один файл для скачивания', 'warning')
    return
  }

  // Automatically include cover images for any folder where tracks were selected
  for (const group of folderGroups.value) {
    const hasSelectedTrack = group.files.some(t => chosenPaths.has(t.path))
    if (hasSelectedTrack && group.coverFiles.length > 0) {
      for (const cov of group.coverFiles) {
        chosenPaths.add(cov.path)
      }
    }
  }

  const paths = Array.from(chosenPaths)
  const hash = pendingInfoHash.value
  if (!hash) {
    dialog.toast('Ошибка: хэш торрента не найден', 'error')
    return
  }

  try {
    pendingInfoHash.value = ''
    showFileModal.value = false
    await downloadsStore.downloadSelectedFiles(hash, paths)
    dialog.toast(`Загрузка начата! Выбрано файлов: ${paths.length}`, 'success')
  } catch (e: any) {
    await dialog.alert({
      title: 'Ошибка запуска загрузки',
      message: e?.message || String(e),
      type: 'error'
    })
  }
}

async function confirmDownloadAll() {
  const hash = pendingInfoHash.value
  if (!hash) {
    dialog.toast('Ошибка: хэш торрента не найден', 'error')
    return
  }

  try {
    pendingInfoHash.value = ''
    showFileModal.value = false
    await downloadsStore.downloadSelectedFiles(hash, [])
    dialog.toast('Загрузка всех файлов раздачи начата!', 'success')
  } catch (e: any) {
    await dialog.alert({
      title: 'Ошибка запуска загрузки',
      message: e?.message || String(e),
      type: 'error'
    })
  }
}

async function streamTrack(file: TorrentFile) {
  try {
    await PlayTrack(pendingInfoHash.value, file.path)
  } catch (e: any) {
    await dialog.alert({
      title: 'Ошибка воспроизведения',
      message: e?.message || String(e),
      type: 'error'
    })
  }
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 ** 2) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 ** 3) return (bytes / 1024 ** 2).toFixed(1) + ' MB'
  return (bytes / 1024 ** 3).toFixed(2) + ' GB'
}

function sourceLabel(source: string) {
  if (source === 'piratebay') return '☠️ PirateBay'
  if (source === '1337x') return '⚡ 1337x'
  return `📦 ${source}`
}

function sourceBadgeClass(source: string) {
  if (source === 'piratebay') return 'badge-pb'
  if (source === '1337x') return 'badge-x'
  return 'badge-source'
}
</script>

<template>
  <div class="search-view">
    <div class="view-header">
      <h1>Поиск музыки</h1>
      <div class="header-actions">
        <button class="btn btn-ghost" @click="showMagnetInput = !showMagnetInput">🔗 Magnet</button>
        <button
          class="btn btn-accent"
          @click="showSourcesModal = true"
          title="Управление JSON источниками загрузок"
        >
          📦 Источники ({{ sourcesList.length }})
        </button>
      </div>
    </div>

    <!-- Magnet input -->
    <div class="magnet-row" v-if="showMagnetInput">
      <input
        v-model="magnetUri"
        type="text"
        placeholder="magnet:?xt=urn:btih:..."
        class="magnet-input"
        @keyup.enter="addMagnetManual"
      />
      <button class="btn btn-accent" @click="addMagnetManual">Добавить</button>
    </div>

    <!-- Search bar -->
    <div class="search-bar">
      <input
        v-model="query"
        type="text"
        placeholder="Исполнитель, альбом, дискография, трек..."
        class="search-input"
        @keyup.enter="doSearch"
      />
      <button class="btn btn-accent search-btn" @click="doSearch" :disabled="loading">
        {{ loading ? 'Ищу...' : '🔍 Найти' }}
      </button>
    </div>

    <p class="error-msg" v-if="error">{{ error }}</p>

    <!-- Results -->
    <div class="results" v-if="results.length">
      <div class="results-count">Найдено раздач: {{ results.length }}</div>
      <div class="result-item" v-for="(r, i) in results" :key="i">
        <div class="result-main">
          <span :class="['badge', sourceBadgeClass(r.source)]">{{ sourceLabel(r.source) }}</span>
          <div class="result-title" :title="r.title">{{ r.title }}</div>
        </div>
        <div class="result-meta">
          <span class="meta-item seeders" title="Сидеры">🌱 {{ r.seeders }}</span>
          <span class="meta-item leechers" title="Личеры" v-if="r.leechers > 0">⬇ {{ r.leechers }}</span>
          <span class="meta-item size" title="Размер">{{ r.size }}</span>
        </div>
        <button class="btn btn-accent btn-sm" @click="openFileSelector(r)">
          📂 Выбрать файлы / Скачать
        </button>
      </div>
    </div>

    <div class="empty-state" v-else-if="!loading">
      <div class="empty-icon">🎵</div>
      <p>Ищите музыку по добавленным источникам (Hydra-style) и торрент-трекерам</p>
      <p class="empty-hint">Нажмите «📦 Источники», чтобы добавить ссылки на каталоги раздач</p>
    </div>

    <!-- Sources Manager Modal (Hydra style) -->
    <Teleport to="body">
      <div class="modal-overlay" v-if="showSourcesModal" @click.self="showSourcesModal = false">
        <div class="modal sources-modal">
          <div class="modal-header">
            <div>
              <h2>📦 Источники загрузки (JSON)</h2>
              <p class="modal-hint">
                Добавляйте ссылки на JSON-источники (например, GitHub Raw). Раздачи индексируются в базе и доступны в мгновенном поиске.
              </p>
            </div>
            <button class="close-btn" @click="showSourcesModal = false">✕</button>
          </div>

          <!-- Add Source Input -->
          <div class="add-source-row">
            <input
              v-model="newSourceUrl"
              type="text"
              placeholder="https://raw.githubusercontent.com/.../rutracker.json"
              class="source-input"
              @keyup.enter="addSourceHandler"
            />
            <button class="btn btn-accent" @click="addSourceHandler" :disabled="addingSource">
              {{ addingSource ? 'Индексация...' : '+ Добавить' }}
            </button>
          </div>

          <p class="error-msg" v-if="sourceError">{{ sourceError }}</p>

          <!-- List of Sources -->
          <div class="sources-list-container">
            <div class="sources-header-bar">
              <span class="sources-title-label">Подключенные каталоги ({{ sourcesList.length }})</span>
              <button
                class="btn btn-ghost btn-xs"
                @click="syncSourcesHandler"
                :disabled="addingSource || sourcesList.length === 0"
              >
                🔄 Обновить все
              </button>
            </div>

            <div v-if="sourcesList.length === 0" class="empty-sources">
              <p>Нет добавленных источников.</p>
              <p class="sub">Вставьте ссылку на JSON со списком раздач выше.</p>
            </div>

            <div v-else class="sources-items">
              <div v-for="s in sourcesList" :key="s.id" class="source-item">
                <div class="source-info">
                  <div class="source-name">
                    <b>{{ s.name }}</b>
                    <span class="source-count">({{ s.itemCount }} раздач)</span>
                  </div>
                  <div class="source-url" :title="s.url">{{ s.url }}</div>
                </div>
                <div class="source-actions">
                  <button class="ctrl-btn-del" @click="removeSourceHandler(s.id)" title="Удалить источник">
                    ✕
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- File Selection Modal -->
    <Teleport to="body">
      <div class="modal-overlay" v-if="showFileModal" @click.self="closeFilePicker">
        <div class="modal file-picker-modal">
          <div class="modal-header">
            <div class="file-modal-titles">
              <h2>Выбор файлов из раздачи</h2>
              <div class="torrent-name-preview" :title="pendingTitle">{{ pendingTitle }}</div>
            </div>
            <button class="close-btn" @click="closeFilePicker">✕</button>
          </div>

          <!-- Loading state -->
          <div class="file-modal-loading" v-if="loadingFiles">
            <div class="spinner">⏳</div>
            <p>Получение списка файлов и метаданных торрента...</p>
            <p class="empty-hint">Пожалуйста, подождите несколько секунд</p>
          </div>

          <!-- File list state -->
          <template v-else>
            <div class="file-filters-bar">
              <input
                v-model="fileSearch"
                type="text"
                placeholder="Фильтр по названию трека / альбому..."
                class="file-search-input"
              />
              <label class="filter-checkbox">
                <input type="checkbox" v-model="audioOnlyFilter" />
                <span>Только аудио</span>
              </label>
              <div class="quick-select-actions">
                <button class="btn btn-ghost btn-xs" @click="selectAllFiltered">Выбрать все</button>
                <button class="btn btn-ghost btn-xs" @click="deselectAll">Снять все</button>
                <button class="btn btn-ghost btn-xs" @click="expandAllFolders" title="Развернуть все альбомы">Развернуть все</button>
                <button class="btn btn-ghost btn-xs" @click="collapseAllFolders" title="Свернуть все альбомы">Свернуть все</button>
              </div>
            </div>

            <!-- Scrollable Folder Groups Accordion -->
            <div class="files-container">
              <div
                v-for="group in folderGroups"
                :key="group.key"
                class="folder-group"
                :class="{ 'is-collapsed': !!collapsedFolders[group.key] }"
              >
                <!-- Folder Header -->
                <div
                  class="folder-header"
                  @click="toggleCollapse(group.key)"
                  :class="{ 'has-selected': getFolderSelectedCount(group) > 0 }"
                >
                  <label class="row-checkbox" @click.stop>
                    <input
                      type="checkbox"
                      :checked="isFolderSelected(group)"
                      :indeterminate.prop="isFolderIndeterminate(group)"
                      @change="toggleFolderSelection(group)"
                    />
                  </label>

                  <div class="folder-title-wrap">
                    <span class="folder-icon">📁</span>
                    <span class="folder-name" :title="group.displayName">{{ group.displayName }}</span>
                  </div>

                  <div class="folder-meta">
                    <!-- Cover auto-download badge -->
                    <span
                      v-if="group.hasCover"
                      class="cover-badge"
                      :title="'Обложка ' + group.coverName + ' скачается автоматически вместе с выбранными треками'"
                    >
                      🖼️ Обложка (авто)
                    </span>

                    <span class="folder-count">{{ group.files.length }} треков</span>
                    <span class="folder-size">{{ formatBytes(group.totalSize) }}</span>

                    <span
                      v-if="getFolderSelectedCount(group) > 0"
                      class="folder-selected-pill"
                    >
                      ✓ {{ getFolderSelectedCount(group) }}/{{ group.files.length }}
                    </span>

                    <span class="accordion-arrow">
                      {{ collapsedFolders[group.key] ? '▶' : '▼' }}
                    </span>
                  </div>
                </div>

                <!-- Folder Track List (Collapsible) -->
                <div class="folder-tracks" v-show="!collapsedFolders[group.key]">
                  <div
                    v-for="f in group.files"
                    :key="f.path"
                    class="file-select-row"
                    :class="{ selected: selectedFiles[f.path] }"
                  >
                    <label class="row-checkbox">
                      <input type="checkbox" v-model="selectedFiles[f.path]" />
                    </label>
                    <div class="file-row-details">
                      <div class="file-row-name" :title="f.path">
                        <span class="music-icon">{{ isAudio(f.name) ? '🎵' : (isCover(f.name) ? '🖼️' : '📄') }}</span>
                        {{ f.name }}
                      </div>
                    </div>
                    <div class="file-row-size">{{ formatBytes(f.size) }}</div>
                    <button
                      v-if="isAudio(f.name)"
                      class="btn btn-ghost btn-xs stream-btn"
                      @click="streamTrack(f)"
                      title="Слушать онлайн прямо сейчас"
                    >
                      ▶ Слушать
                    </button>
                  </div>
                </div>
              </div>

              <div class="empty-file-filter" v-if="folderGroups.length === 0">
                Файлы не найдены по запросу
              </div>
            </div>

            <!-- Modal Footer -->
            <div class="file-modal-footer">
              <div class="footer-stats">
                <b>Выбрано треков:</b> {{ selectedCount }} из {{ totalAudioFilesCount }}
                <span class="footer-size">({{ selectedSize }} из {{ totalFilesSize }})</span>
                <span class="footer-cover-hint">🖼️ Обложки скачиваются автоматически</span>
              </div>
              <div class="footer-actions">
                <button class="btn btn-ghost" @click="closeFilePicker">Отмена</button>
                <button class="btn btn-ghost" @click="confirmDownloadAll">Скачать всё</button>
                <button
                  class="btn btn-accent"
                  :disabled="selectedCount === 0"
                  @click="confirmDownloadSelected"
                >
                  ⬇ Скачать выбранные ({{ selectedCount }})
                </button>
              </div>
            </div>
          </template>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.search-view { display: flex; flex-direction: column; gap: 20px; }

.view-header { display: flex; justify-content: space-between; align-items: center; }
.view-header h1 { font-size: 24px; font-weight: 700; }
.header-actions { display: flex; gap: 10px; }

.magnet-row { display: flex; gap: 10px; }
.magnet-input { flex: 1; }

.search-bar { display: flex; gap: 12px; }
.search-input { flex: 1; font-size: 15px; }
.search-btn { padding: 9px 20px; font-size: 14px; }

.error-msg { color: var(--danger); font-size: 13px; }

.results { display: flex; flex-direction: column; gap: 6px; }
.results-count { font-size: 13px; color: var(--text2); margin-bottom: 4px; }

.result-item {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  background: var(--bg3);
  border-radius: 10px;
  border: 1px solid transparent;
  transition: border-color 0.12s, background 0.12s;
}
.result-item:hover { border-color: var(--accent); background: var(--surface); }

.result-main { display: flex; align-items: center; gap: 10px; min-width: 0; }
.result-title { font-size: 14px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.badge { font-size: 11px; padding: 2px 8px; border-radius: 4px; white-space: nowrap; flex-shrink: 0; font-weight: 600; }
.badge-pb { background: #3a1a1a; color: #f44336; }
.badge-x  { background: #1a1a3a; color: #2196f3; }
.badge-source { background: #1a3a2a; color: #2ecc71; border: 1px solid rgba(46, 204, 113, 0.3); }

.result-meta { display: flex; gap: 12px; font-size: 13px; color: var(--text2); white-space: nowrap; }
.seeders { color: #4caf50; font-weight: 600; }

.btn-sm { padding: 6px 14px; font-size: 13px; white-space: nowrap; }
.btn-xs { padding: 4px 10px; font-size: 12px; }

.empty-state { text-align: center; padding: 60px 0; color: var(--text2); }
.empty-icon { font-size: 56px; margin-bottom: 16px; }
.empty-hint { font-size: 13px; margin-top: 8px; color: var(--text2); opacity: 0.6; }

/* Modals */
.modal-overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.75);
  display: flex; align-items: center; justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(4px);
}
.modal {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 24px;
  display: flex; flex-direction: column; gap: 14px;
  box-shadow: 0 10px 40px rgba(0,0,0,0.6);
}
.modal-header {
  display: flex; justify-content: space-between; align-items: flex-start; gap: 12px;
}
.close-btn {
  font-size: 18px; opacity: 0.6; padding: 4px 8px; border-radius: 6px;
}
.close-btn:hover { opacity: 1; background: var(--surface); }

/* Sources Modal */
.sources-modal { width: 560px; max-width: 90vw; }
.add-source-row { display: flex; gap: 10px; }
.source-input { flex: 1; font-size: 13px; }
.sources-list-container {
  display: flex; flex-direction: column; gap: 8px; margin-top: 4px;
}
.sources-header-bar {
  display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--text2);
}
.empty-sources {
  text-align: center; padding: 30px; color: var(--text2); font-size: 13px; background: var(--bg3); border-radius: 8px;
}
.empty-sources .sub { font-size: 11px; opacity: 0.7; margin-top: 4px; }
.sources-items {
  max-height: 240px; overflow-y: auto; display: flex; flex-direction: column; gap: 6px;
}
.source-item {
  display: flex; justify-content: space-between; align-items: center;
  background: var(--bg3); padding: 10px 14px; border-radius: 8px; gap: 12px;
}
.source-info { min-width: 0; flex: 1; }
.source-name { font-size: 13px; display: flex; align-items: baseline; gap: 6px; }
.source-count { font-size: 11px; color: var(--accent); }
.source-url { font-size: 11px; color: var(--text2); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-family: monospace; margin-top: 2px; }
.ctrl-btn-del {
  font-size: 14px; opacity: 0.5; padding: 4px 8px; border-radius: 6px;
}
.ctrl-btn-del:hover { opacity: 1; background: var(--danger); color: #fff; }

/* File Picker Modal */
.file-picker-modal {
  width: 980px;
  max-width: 96vw;
  height: 90vh;
  max-height: 850px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 18px 22px;
  gap: 12px;
}
.file-modal-titles { min-width: 0; }
.file-modal-titles h2 { font-size: 18px; font-weight: 700; }
.torrent-name-preview {
  font-size: 13px; color: var(--accent); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 800px;
  margin-top: 2px;
}
.file-modal-loading {
  flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px;
}
.spinner { font-size: 40px; animation: pulse 1s infinite alternate; }
@keyframes pulse { from { opacity: 0.5; } to { opacity: 1; } }

.file-filters-bar {
  display: flex; align-items: center; gap: 12px; background: var(--bg3); padding: 8px 12px; border-radius: 8px; flex-shrink: 0;
}
.file-search-input { flex: 1; font-size: 13px; padding: 6px 12px; }
.filter-checkbox { display: flex; align-items: center; gap: 6px; font-size: 13px; cursor: pointer; white-space: nowrap; }
.quick-select-actions { display: flex; gap: 6px; flex-shrink: 0; }

.files-container {
  flex: 1 1 0;
  min-height: 0; /* CRITICAL: Enables flex child shrinking and scrollbar */
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 10px;
}

/* Folder Groups */
.folder-group {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  overflow: hidden;
  flex-shrink: 0;
}

.folder-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  background: rgba(255, 255, 255, 0.05);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s;
}

.folder-group.is-collapsed .folder-header {
  border-bottom: none;
}

.folder-header:hover {
  background: rgba(255, 255, 255, 0.09);
}

.folder-header.has-selected {
  background: rgba(29, 185, 84, 0.1);
}

.folder-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.folder-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.folder-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.folder-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text2);
  flex-shrink: 0;
}

.cover-badge {
  background: rgba(16, 185, 129, 0.18);
  color: #34d399;
  padding: 2px 7px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
}

.folder-count, .folder-size {
  color: var(--text2);
  font-size: 11px;
}

.folder-selected-pill {
  background: rgba(29, 185, 84, 0.2);
  color: #1ed760;
  padding: 2px 7px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
}

.accordion-arrow {
  font-size: 10px;
  color: var(--text2);
  padding: 2px 4px;
}

.folder-tracks {
  padding: 4px 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.file-select-row {
  display: grid;
  grid-template-columns: 24px 1fr auto auto;
  align-items: center;
  gap: 12px;
  padding: 7px 10px;
  border-radius: 6px;
  transition: background 0.1s;
}
.file-select-row:hover { background: rgba(255, 255, 255, 0.05); }
.file-select-row.selected { background: rgba(29, 185, 84, 0.09); }
.row-checkbox { display: flex; align-items: center; cursor: pointer; }
.row-checkbox input { width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer; }
.file-row-details { min-width: 0; }
.file-row-name { font-size: 13px; font-weight: 400; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: flex; align-items: center; gap: 6px; }
.music-icon { font-size: 12px; flex-shrink: 0; }
.file-row-size { font-size: 12px; color: var(--text2); white-space: nowrap; }
.stream-btn { color: var(--accent); }
.stream-btn:hover { background: var(--surface2); }
.empty-file-filter { text-align: center; padding: 40px; color: var(--text2); font-size: 13px; }

.file-modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-top: 1px solid var(--border);
  padding-top: 10px;
  flex-shrink: 0;
}
.footer-stats { font-size: 13px; color: var(--text); display: flex; align-items: center; gap: 6px; }
.footer-size { color: var(--text2); }
.footer-cover-hint {
  font-size: 11px;
  color: #34d399;
  background: rgba(16, 185, 129, 0.12);
  padding: 2px 8px;
  border-radius: 12px;
  margin-left: 8px;
}
.footer-actions { display: flex; gap: 10px; }
</style>
