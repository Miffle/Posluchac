<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { UpdateTrackMetadata } from '../../wailsjs/go/main/App'
import { useDialogStore } from '../store/dialog'
import { useI18n } from '../i18n'

const props = defineProps<{
  track: any | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', track: any): void
}>()

const dialog = useDialogStore()
const { t } = useI18n()

const title = ref('')
const artist = ref('')
const album = ref('')
const year = ref<number | ''>('')
const genre = ref('')
const writeToFile = ref(false)
const saving = ref(false)
const error = ref('')

const isMp3 = computed(() => {
  const p = props.track?.filePath || props.track?.fileName || ''
  return p.toLowerCase().endsWith('.mp3')
})

watch(() => props.track, (tr) => {
  if (tr) {
    title.value = tr.title || ''
    artist.value = tr.artist || ''
    album.value = tr.album || ''
    year.value = tr.year > 0 ? tr.year : ''
    genre.value = tr.genre || ''
    writeToFile.value = false
    error.value = ''
  }
}, { immediate: true })

async function handleSave() {
  if (!props.track || saving.value) return
  saving.value = true
  error.value = ''

  try {
    const y = typeof year.value === 'number' ? year.value : (parseInt(String(year.value), 10) || 0)
    const updated = await UpdateTrackMetadata(
      props.track.id,
      title.value,
      artist.value,
      album.value,
      y,
      genre.value,
      writeToFile.value
    )
    dialog.toast(t('library.toastMetadataUpdated'), 'success')
    emit('saved', updated)
    emit('close')
  } catch (err: any) {
    error.value = err?.message || String(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Teleport to="body">
  <div class="modal-backdrop" v-if="track" @click.self="emit('close')">
    <div class="modal-card">
      <div class="modal-header">
        <div class="header-title">
          <span class="icon">🏷️</span>
          <h3>{{ t('library.editMetadataTitle') }}</h3>
        </div>
        <button class="close-btn" @click="emit('close')" :title="t('common.close')">✕</button>
      </div>

      <div class="track-summary">
        <div class="summary-file" :title="track.filePath || track.fileName">
          📄 {{ track.fileName }}
        </div>
      </div>

      <form class="metadata-form" @submit.prevent="handleSave">
        <!-- Title -->
        <div class="form-group">
          <label class="form-label">{{ t('library.trackTitle') }}</label>
          <input
            v-model="title"
            type="text"
            class="form-input"
            :placeholder="t('library.trackTitle')"
            autofocus
          />
        </div>

        <!-- Artist -->
        <div class="form-group">
          <label class="form-label">{{ t('library.trackArtist') }}</label>
          <input
            v-model="artist"
            type="text"
            class="form-input"
            :placeholder="t('library.trackArtist')"
          />
        </div>

        <!-- Album -->
        <div class="form-group">
          <label class="form-label">{{ t('library.trackAlbum') }}</label>
          <input
            v-model="album"
            type="text"
            class="form-input"
            :placeholder="t('library.trackAlbum')"
          />
        </div>

        <!-- Year & Genre Row -->
        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">{{ t('library.trackYear') }}</label>
            <input
              v-model="year"
              type="number"
              min="1900"
              max="2100"
              class="form-input"
              placeholder="2024"
            />
          </div>
          <div class="form-group flex-2">
            <label class="form-label">{{ t('library.trackGenre') }}</label>
            <input
              v-model="genre"
              type="text"
              class="form-input"
              placeholder="Rock, Pop, Jazz..."
            />
          </div>
        </div>

        <!-- Option to write to disk -->
        <div class="checkbox-group" v-if="isMp3">
          <label class="checkbox-label">
            <input type="checkbox" v-model="writeToFile" />
            <span>{{ t('library.writeToDisk') }}</span>
          </label>
          <div class="checkbox-hint">{{ t('library.writeToDiskHint') }}</div>
        </div>

        <div v-if="error" class="error-msg">
          {{ error }}
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn btn-accent" :disabled="saving">
            {{ saving ? t('common.loading') : t('common.save') }}
          </button>
        </div>
      </form>
    </div>
  </div>
  </Teleport>
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
  z-index: 10000;
  padding: 20px;
}

.modal-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: 100%;
  max-width: 480px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  animation: modal-in 0.15s ease-out;
}

@keyframes modal-in {
  from {
    opacity: 0;
    transform: scale(0.96) translateY(-8px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-title .icon {
  font-size: 20px;
}

.header-title h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  margin: 0;
}

.close-btn {
  background: transparent;
  border: none;
  color: var(--text2);
  font-size: 16px;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  transition: all 0.15s;
}

.close-btn:hover {
  color: var(--text);
  background: var(--surface);
}

.track-summary {
  padding: 12px 20px;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
}

.summary-file {
  font-size: 13px;
  color: var(--text2);
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metadata-form {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }
.flex-2 { flex: 2; }

.form-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text2);
}

.form-input {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--text);
  font-size: 14px;
  transition: border-color 0.15s;
}

.form-input:focus {
  outline: none;
  border-color: var(--accent);
}

.checkbox-group {
  margin-top: 4px;
  padding: 10px 12px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.checkbox-hint {
  font-size: 11px;
  color: var(--text2);
  line-height: 1.4;
  padding-left: 22px;
}

.error-msg {
  padding: 8px 12px;
  background: rgba(231, 76, 60, 0.15);
  color: #ff6b6b;
  border-radius: 6px;
  font-size: 13px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

.btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  transition: all 0.15s ease;
}

.btn-secondary {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
}

.btn-secondary:hover {
  background: var(--surface2);
}

.btn-accent {
  background: var(--accent);
  color: #000;
}

.btn-accent:hover:not(:disabled) {
  opacity: 0.9;
  transform: translateY(-1px);
}

.btn-accent:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
